// helpers_test.go 覆盖自动化服务的公共测试夹具与测试专用入口。
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

// noMainWindow 给只走隔离会话的测试 runner 补齐 dmRunner：这些测试不应触达主窗口。
type noMainWindow struct{}

func (noMainWindow) AppendMainWindowTask(context.Context, string, dmsvc.MainWindowTask) (string, error) {
	return "", errors.New("main window is not used by this test runner")
}

type fakeDMRunner struct {
	permission               *permissionctx.Context
	resultText               string
	assistantText            string
	delay                    time.Duration
	requiredTool             string
	requiredTools            []string
	skipPermissionAfterFirst bool

	mu         sync.Mutex
	requests   []dmsvc.Request
	interrupts []dmsvc.InterruptRequest
	mainTasks  []dmsvc.MainWindowTask
	mainErr    error
}

// AppendMainWindowTask 记录写入主窗口的任务；mainErr 模拟主窗口不可用。
func (f *fakeDMRunner) AppendMainWindowTask(_ context.Context, agentID string, task dmsvc.MainWindowTask) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mainErr != nil {
		return "", f.mainErr
	}
	f.mainTasks = append(f.mainTasks, task)
	return "agent:" + agentID + ":ws:dm:conv_main", nil
}

func (f *fakeDMRunner) HandleChat(_ context.Context, request dmsvc.Request) error {
	f.mu.Lock()
	requestIndex := len(f.requests)
	f.requests = append(f.requests, request)
	f.mu.Unlock()

	go func() {
		delay := f.delay
		if delay <= 0 {
			delay = 20 * time.Millisecond
		}
		time.Sleep(delay)
		emit := func(event protocol.EventMessage) {
			f.permission.BroadcastEvent(context.Background(), request.SessionKey, event)
		}
		if !(f.skipPermissionAfterFirst && requestIndex > 0) &&
			f.emitPermissionDeniedResult(context.Background(), request, emit) {
			return
		}
		emit(protocol.EventMessage{
			ProtocolVersion: 2,
			DeliveryMode:    "durable",
			EventType:       protocol.EventTypeMessage,
			SessionKey:      request.SessionKey,
			Data: map[string]any{
				"message_id": "assistant_" + request.RoundID,
				"round_id":   request.RoundID,
				"role":       "assistant",
				"session_id": "sdk_" + request.RoundID,
				"content": []map[string]any{
					{
						"type": "text",
						"text": textutil.FirstNonEmpty(f.assistantText, f.resultText, "ok"),
					},
				},
			},
			Timestamp: time.Now().UnixMilli(),
		})
		emit(protocol.EventMessage{
			ProtocolVersion: 2,
			DeliveryMode:    "durable",
			EventType:       protocol.EventTypeMessage,
			SessionKey:      request.SessionKey,
			Data: map[string]any{
				"message_id": "result_" + request.RoundID,
				"round_id":   request.RoundID,
				"role":       "result",
				"subtype":    "success",
				"result":     textutil.FirstNonEmpty(f.resultText, "ok"),
				"session_id": "sdk_" + request.RoundID,
			},
			Timestamp: time.Now().UnixMilli(),
		})
		emit(protocol.NewRoundStatusEvent(
			request.SessionKey,
			request.RoundID,
			"finished",
			"success",
		))
	}()
	return nil
}

func (f *fakeDMRunner) emitPermissionDeniedResult(
	ctx context.Context,
	request dmsvc.Request,
	emit func(protocol.EventMessage),
) bool {
	toolNames := slices.Clone(f.requiredTools)
	if len(toolNames) == 0 && strings.TrimSpace(f.requiredTool) != "" {
		toolNames = []string{f.requiredTool}
	}
	if len(toolNames) == 0 || request.PermissionHandler == nil {
		return false
	}
	for _, toolName := range toolNames {
		toolName = strings.TrimSpace(toolName)
		decision, err := request.PermissionHandler(ctx, sdkpermission.Request{ToolName: toolName})
		if err != nil {
			decision = sdkpermission.Deny(err.Error(), false)
		}
		if decision.Behavior == sdkpermission.BehaviorAllow {
			continue
		}
		f.emitPermissionDenial(request, emit, toolName, decision)
		return true
	}
	return false
}

func (f *fakeDMRunner) emitPermissionDenial(
	request dmsvc.Request,
	emit func(protocol.EventMessage),
	toolName string,
	decision sdkpermission.Decision,
) {
	message := textutil.FirstNonEmpty(
		decision.Message,
		"当前 Agent 未授权工具 "+toolName+"；请先在 Agent 允许工具中配置该工具，或把任务改为无需该工具",
	)
	emit(protocol.EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    "durable",
		EventType:       protocol.EventTypeMessage,
		SessionKey:      request.SessionKey,
		Data: map[string]any{
			"message_id": "result_" + request.RoundID,
			"round_id":   request.RoundID,
			"role":       "result",
			"subtype":    "success",
			"result":     message,
			"session_id": "sdk_" + request.RoundID,
			"permission_denials": []map[string]any{{
				"tool_name": toolName,
			}},
		},
		Timestamp: time.Now().UnixMilli(),
	})
	emit(protocol.NewRoundStatusEvent(
		request.SessionKey,
		request.RoundID,
		"finished",
		"success",
	))
}

func (f *fakeDMRunner) Requests() []dmsvc.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]dmsvc.Request, len(f.requests))
	copy(result, f.requests)
	return result
}

func (f *fakeDMRunner) HandleInterrupt(_ context.Context, request dmsvc.InterruptRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interrupts = append(f.interrupts, request)
	return nil
}

func (f *fakeDMRunner) Interrupts() []dmsvc.InterruptRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]dmsvc.InterruptRequest, len(f.interrupts))
	copy(result, f.interrupts)
	return result
}

type fakeRoomRunner struct {
	permission *permissionctx.Context
	resultText string
	delay      time.Duration
	contexts   map[string]*protocol.ConversationContextAggregate

	mu         sync.Mutex
	requests   []roomrealtime.ChatRequest
	interrupts []roomrealtime.InterruptRequest
	err        error
}

func (f *fakeRoomRunner) GetConversationContext(_ context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	conversationID = strings.TrimSpace(conversationID)
	if f.contexts != nil {
		if contextValue := f.contexts[conversationID]; contextValue != nil {
			return contextValue, nil
		}
	}
	return &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup, HostAgentID: "agent-1"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-1"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-1", Name: "Agent 1"},
		},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: "room-1"},
		Sessions: []protocol.SessionRecord{
			{ConversationID: conversationID, AgentID: "agent-1", Status: "active"},
		},
	}, nil
}

func (f *fakeRoomRunner) HandleChat(_ context.Context, request roomrealtime.ChatRequest) error {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	err := f.err
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if f.permission == nil && request.EventObserver == nil {
		return nil
	}
	go func() {
		delay := f.delay
		if delay <= 0 {
			delay = 20 * time.Millisecond
		}
		time.Sleep(delay)
		emit := func(event protocol.EventMessage) {
			if request.EventObserver != nil {
				request.EventObserver(context.Background(), event)
				return
			}
			f.permission.BroadcastEvent(context.Background(), request.SessionKey, event)
		}
		emit(protocol.EventMessage{
			ProtocolVersion: 2,
			DeliveryMode:    "durable",
			EventType:       protocol.EventTypeMessage,
			SessionKey:      request.SessionKey,
			Data: map[string]any{
				"message_id": "assistant_" + request.RoundID,
				"round_id":   request.RoundID,
				"role":       "assistant",
				"session_id": "sdk_" + request.RoundID,
				"content": []map[string]any{
					{
						"type": "text",
						"text": textutil.FirstNonEmpty(f.resultText, "room ok"),
					},
				},
			},
			Timestamp: time.Now().UnixMilli(),
		})
		emit(protocol.EventMessage{
			ProtocolVersion: 2,
			DeliveryMode:    "durable",
			EventType:       protocol.EventTypeMessage,
			SessionKey:      request.SessionKey,
			Data: map[string]any{
				"message_id": "result_" + request.RoundID,
				"round_id":   request.RoundID,
				"role":       "result",
				"subtype":    "success",
				"result":     textutil.FirstNonEmpty(f.resultText, "room ok"),
				"session_id": "sdk_" + request.RoundID,
			},
			Timestamp: time.Now().UnixMilli(),
		})
		emit(protocol.NewRoundStatusEvent(
			request.SessionKey,
			request.RoundID,
			"finished",
			"success",
		))
	}()
	return nil
}

func (f *fakeRoomRunner) Requests() []roomrealtime.ChatRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]roomrealtime.ChatRequest, len(f.requests))
	copy(result, f.requests)
	return result
}

func (f *fakeRoomRunner) HandleInterrupt(_ context.Context, request roomrealtime.InterruptRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interrupts = append(f.interrupts, request)
	return nil
}

func (f *fakeRoomRunner) Interrupts() []roomrealtime.InterruptRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]roomrealtime.InterruptRequest, len(f.interrupts))
	copy(result, f.interrupts)
	return result
}

type fakeWorkspaceReader struct {
	files map[string]string
}

func (f *fakeWorkspaceReader) GetFile(_ context.Context, _ string, relativePath string) (*workspacepkg.FileContent, error) {
	content, ok := f.files[relativePath]
	if !ok {
		return nil, workspacepkg.ErrFileNotFound
	}
	return &workspacepkg.FileContent{
		Path:    relativePath,
		Content: content,
	}, nil
}

type fakeDeliveryRouter struct {
	mu           sync.Mutex
	calls        []channels.DeliveryTarget
	agentIDs     []string
	messages     []string
	ownerUserIDs []string
	err          error
	receipt      *channelmessage.Receipt
}

func fakeStructuredDelivery(agentID string, ref string) automationdomain.DeliveryTarget {
	sessionKey := protocol.BuildAgentSessionKey(
		agentID,
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		ref,
		"",
	)
	return automationdomain.DeliveryTarget{
		Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
		To: sessionKey, SessionKey: sessionKey,
	}
}

func (f *fakeDeliveryRouter) DeliverMessage(
	ctx context.Context,
	agentID string,
	text string,
	target channels.DeliveryTarget,
) (channels.DeliveryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, target)
	f.agentIDs = append(f.agentIDs, strings.TrimSpace(agentID))
	f.messages = append(f.messages, text)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
	if f.err != nil {
		return channels.DeliveryResult{}, f.err
	}
	return channels.DeliveryResult{Target: target, Receipt: f.receipt}, nil
}

func (f *fakeDeliveryRouter) AgentIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.agentIDs)
}

func (f *fakeDeliveryRouter) Messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.messages)
}

func (f *fakeDeliveryRouter) Calls() []channels.DeliveryTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]channels.DeliveryTarget, len(f.calls))
	copy(result, f.calls)
	return result
}

func (f *fakeDeliveryRouter) OwnerUserIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]string, len(f.ownerUserIDs))
	copy(result, f.ownerUserIDs)
	return result
}

func newAutomationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	schema := `
CREATE TABLE agents (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT 'user-1'
);
CREATE TABLE automation_scheduled_tasks (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__',
    name VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    source_kind VARCHAR(32) NOT NULL DEFAULT 'manual',
    source_creator_agent_id VARCHAR(64),
    source_context_type VARCHAR(64),
    source_context_id VARCHAR(64),
    source_context_label VARCHAR(255),
    source_session_key VARCHAR(255),
    source_session_label VARCHAR(255),
    overlap_policy VARCHAR(32) NOT NULL DEFAULT 'skip',
    expires_at DATETIME,
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
    execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent',
    permission_mode VARCHAR(32) NOT NULL DEFAULT 'default',
    session_target_kind VARCHAR(32) NOT NULL,
    bound_session_key VARCHAR(255),
    named_session_key VARCHAR(255),
    wake_mode VARCHAR(32) NOT NULL,
    delivery_mode VARCHAR(32) NOT NULL,
    delivery_channel VARCHAR(64),
    delivery_to VARCHAR(255),
    delivery_account_id VARCHAR(64),
    delivery_thread_id VARCHAR(255),
    delivery_session_key VARCHAR(255),
    delivery_agent_id VARCHAR(64),
    session_binding_state VARCHAR(32) NOT NULL DEFAULT 'ready',
    invalidated_session_keys_json TEXT NOT NULL DEFAULT '[]',
	delivery_grant_json TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL,
    next_run_at DATETIME,
    running_run_id VARCHAR(64),
    running_started_at DATETIME,
    last_run_at DATETIME,
    last_run_status VARCHAR(32),
    failure_streak INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    last_delivery_status VARCHAR(32),
	last_completed_run_id VARCHAR(64),
    configuration_version INTEGER NOT NULL DEFAULT 1,
    deletion_state VARCHAR(32) NOT NULL DEFAULT '',
    deletion_token VARCHAR(128),
    deletion_claimed_at DATETIME,
    permission_policy_json TEXT NOT NULL DEFAULT '{}',
    permission_policy_revision INTEGER NOT NULL DEFAULT 0,
    permission_state VARCHAR(32) NOT NULL DEFAULT 'uninitialized',
    pending_permission_request_id VARCHAR(64),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_task_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__',
    status VARCHAR(32) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL DEFAULT '',
    session_key VARCHAR(255),
    round_id VARCHAR(64),
    session_id VARCHAR(255),
	    message_count INTEGER NOT NULL DEFAULT 0,
	    delivery_mode VARCHAR(32),
	    delivery_to VARCHAR(255),
	    delivery_target_json TEXT,
	    delivery_status VARCHAR(32),
	    delivery_error TEXT,
	    delivered_at DATETIME,
	    delivery_attempts INTEGER NOT NULL DEFAULT 0,
	    delivery_attempt_id VARCHAR(64),
	    delivery_attempt_started_at DATETIME,
	    delivery_next_attempt_at DATETIME,
	    delivery_dead_letter_at DATETIME,
	    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    result_summary TEXT,
    assistant_text TEXT,
    result_text TEXT,
    artifact_path VARCHAR(512),
    permission_policy_revision INTEGER NOT NULL DEFAULT 0,
    block_state VARCHAR(32) NOT NULL DEFAULT '',
    blocked_request_id VARCHAR(64),
    effect_started BOOLEAN NOT NULL DEFAULT 0,
    client_request_id VARCHAR(128),
    client_intent_digest VARCHAR(64),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX uq_automation_task_runs_owner_request
    ON automation_task_runs (owner_user_id, client_request_id)
    WHERE client_request_id IS NOT NULL AND client_request_id <> '';
CREATE TABLE automation_permission_requests (
    request_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    job_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64),
    policy_revision INTEGER NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    decision VARCHAR(32),
    tool_name VARCHAR(255) NOT NULL,
    connector_id VARCHAR(64),
    effect VARCHAR(32) NOT NULL,
    resource_scope TEXT,
    input_fingerprint VARCHAR(64) NOT NULL,
    capability_json TEXT NOT NULL,
    input_summary_json TEXT NOT NULL DEFAULT '{}',
    title VARCHAR(255),
    description TEXT,
    reason TEXT,
    session_key VARCHAR(255),
    delivery_session_key VARCHAR(255),
    round_id VARCHAR(64),
    tool_use_id VARCHAR(255),
    resume_safe BOOLEAN NOT NULL DEFAULT 1,
    resolved_by_user_id VARCHAR(64),
    resolved_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX uq_automation_permission_requests_pending_capability
    ON automation_permission_requests (owner_user_id, job_id, run_id, kind, input_fingerprint)
    WHERE status = 'pending';
CREATE TABLE automation_scheduler_leases (
    lease_name VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_task_create_requests (
    owner_user_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    job_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    intent_digest VARCHAR(64) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, request_id)
);
CREATE TABLE automation_runtime_commands (
    owner_user_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    actor_agent_id VARCHAR(64) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    intent_digest VARCHAR(64) NOT NULL,
    approval_request_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    result_json TEXT,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, request_id)
);
CREATE TABLE automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL DEFAULT '',
    target_session_key VARCHAR(512) NOT NULL DEFAULT '',
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
    context_token TEXT,
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_system_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    source_type VARCHAR(64),
    source_id VARCHAR(64),
    payload JSON NOT NULL,
    status VARCHAR(32) NOT NULL,
    processed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_task_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,
    actor_user_id VARCHAR(64),
    actor_agent_id VARCHAR(64),
    run_id VARCHAR(64),
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
INSERT INTO agents(id) VALUES ('agent-1');`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化测试 schema 失败: %v", err)
	}
	return db
}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待条件达成超时: %s", timeout)
}

func intRef(value int) *int {
	result := value
	return &result
}

func permissionDecisionInputForRequest(
	request automationdomain.AutomationPermissionRequest,
	decision string,
) automationdomain.PermissionDecisionInput {
	return automationdomain.PermissionDecisionInput{
		Decision:       decision,
		JobID:          request.JobID,
		RunID:          request.RunID,
		PolicyRevision: request.PolicyRevision,
	}
}

func permissionResumeInputForRequest(
	request automationdomain.AutomationPermissionRequest,
) automationdomain.PermissionResumeInput {
	return automationdomain.PermissionResumeInput{
		RequestID:      request.RequestID,
		PolicyRevision: request.PolicyRevision,
	}
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	return slices.ContainsFunc(items, func(item string) bool {
		return strings.TrimSpace(item) == target
	})
}

func newAutomationOwnerWorkspace(t *testing.T, ownerUserID string, agentID string) string {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	return filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		agentID,
	)
}

type testAgentResolver struct {
	workspacePath string
}

func (r *testAgentResolver) GetAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	return &protocol.Agent{
		AgentID:       agentID,
		OwnerUserID:   authctx.OwnerUserID(ctx),
		WorkspacePath: r.workspacePath,
	}, nil
}

func (r *testAgentResolver) GetDefaultAgent(ctx context.Context) (*protocol.Agent, error) {
	return &protocol.Agent{
		AgentID:       "nexus",
		OwnerUserID:   authctx.OwnerUserID(ctx),
		WorkspacePath: r.workspacePath,
		IsMain:        true,
	}, nil
}

func stringFromMessage(message protocol.Message, key string) string {
	if value, ok := message[key].(string); ok {
		return strings.TrimSpace(value)
	}
	if key != "content" {
		return ""
	}
	if items, ok := message[key].([]map[string]any); ok {
		return joinTextBlocks(items)
	}
	rawItems, ok := message[key].([]any)
	if !ok {
		return ""
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		payload, ok := raw.(map[string]any)
		if ok {
			items = append(items, payload)
		}
	}
	return joinTextBlocks(items)
}

func joinTextBlocks(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if textutil.FirstNonEmpty(strings.TrimSpace(messageAnyString(item["type"]))) != "text" {
			continue
		}
		text := strings.TrimSpace(messageAnyString(item["text"]))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func messageAnyString(value any) string {
	text, _ := value.(string)
	return text
}

var automationCommandRequestSequence atomic.Uint64

type automationCommandFixture struct {
	WorkspacePath string
	Permission    *permissionctx.Context
	DM            *fakeDMRunner
	Router        *channels.Router
	Service       *Service
	ServerContext command.Actor
}

type allowAutomationDeliveryGrant struct{}

func (allowAutomationDeliveryGrant) ValidateExternalSessionGrant(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (allowAutomationDeliveryGrant) ListAgentExternalSessions(
	context.Context,
	string,
	string,
	string,
) ([]channels.AgentExternalSession, error) {
	return nil, nil
}

func newAutomationCommandFixture(t *testing.T, resultText string) automationCommandFixture {
	t.Helper()
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: textutil.FirstNonEmpty(resultText, "ok"),
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)
	service.SetDeliveryGrantResolver(allowAutomationDeliveryGrant{})
	currentSessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
		"",
	)
	prepareAutomationDeliverySession(t, workspacePath, "user-1", "agent-1", currentSessionKey)
	return automationCommandFixture{
		WorkspacePath: workspacePath,
		Permission:    permission,
		DM:            dm,
		Router:        router,
		Service:       service,
		ServerContext: command.Actor{
			AgentID:            "agent-1",
			AgentName:          "新闻智能体",
			OwnerUserID:        "user-1",
			SessionKey:         currentSessionKey,
			SessionLabel:       "用户对话",
			SourceContextType:  "agent",
			SourceContextID:    "agent-1",
			SourceContextLabel: "新闻智能体",
			DefaultTimezone:    "Asia/Shanghai",
			RoundID:            "round-test",
			LeaseSessionKey:    currentSessionKey,
			LeaseRoundID:       "round-test",
			Round: nexusmcp.RoundContext{
				CommandReceipts: nexusmcp.NewCommandReceiptState(),
			},
		},
	}
}

func prepareAutomationDeliverySession(
	t *testing.T,
	workspacePath string,
	ownerUserID string,
	agentID string,
	sessionKey string,
) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := workspacestore.NewSessionFileStore(workspacePath).
		ForOwner(ownerUserID).
		UpsertSession(workspacePath, protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      agentID,
			ChannelType:  protocol.NormalizeStoredChannelType(protocol.ParseSessionKey(sessionKey).Channel),
			ChatType:     protocol.RoomTypeDM,
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "用户对话",
			Options:      map[string]any{},
			IsActive:     true,
		}); err != nil {
		t.Fatalf("准备真实接收会话失败: %v", err)
	}
}

func callAutomationCommand(
	t *testing.T,
	service *Service,
	sctx command.Actor,
	name string,
	args map[string]any,
) (map[string]any, bool) {
	t.Helper()
	if strings.TrimSpace(sctx.LeaseSessionKey) == "" {
		sctx.LeaseSessionKey = textutil.FirstNonEmpty(sctx.SessionKey, "agent:test:dm:websocket:test:")
	}
	if strings.TrimSpace(sctx.LeaseRoundID) == "" {
		sctx.LeaseRoundID = textutil.FirstNonEmpty(sctx.RoundID, "round-test")
	}
	if strings.TrimSpace(sctx.RoundID) == "" {
		sctx.RoundID = sctx.LeaseRoundID
	}
	if strings.TrimSpace(sctx.SessionKey) == "" {
		sctx.SessionKey = sctx.LeaseSessionKey
	}
	if sctx.Round.CommandReceipts == nil {
		sctx.Round.CommandReceipts = nexusmcp.NewCommandReceiptState()
	}
	name, args = automationCommandTestRoute(name, args)
	if !sctx.CrossAgentAllowed() {
		if rawMode, ok := args["execution_mode"].(string); ok && strings.TrimSpace(rawMode) != "" {
			mode := strings.TrimSpace(rawMode)
			contextMode := "isolated"
			if mode == "existing" {
				contextMode = "current"
			}
			args["context_mode"] = contextMode
		}
		if rawMode, ok := args["reply_mode"].(string); ok && strings.TrimSpace(rawMode) != "" {
			mode := strings.TrimSpace(rawMode)
			args["deliver_result"] = mode != "none"
		}
		for _, field := range []string{
			"execution_mode", "reply_mode", "selected_session_key", "named_session_key",
			"selected_reply_session_key", "reply_session_key", "reply_channel", "reply_to",
			"reply_account_id", "reply_thread_id",
		} {
			delete(args, field)
		}
	}
	operation := strings.TrimSpace(fmt.Sprint(args["operation"]))
	requestID, _ := args["request_id"].(string)
	requestID = strings.TrimSpace(requestID)
	if operation == "create" {
		_, hasRequestID := args["request_id"]
		if !hasRequestID || requestID == "" {
			requestID = fmt.Sprintf(
				"test-%s-%d",
				strings.ReplaceAll(t.Name(), "/", "-"),
				automationCommandRequestSequence.Add(1),
			)
			args["request_id"] = requestID
		}
	}
	delete(args, "operation")
	delete(args, "request_id")
	delete(args, "view")
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal command input: %v", err)
	}
	var input automationdomain.AutomationCommandInput
	if err = json.Unmarshal(encoded, &input); err != nil {
		t.Fatalf("decode command input: %v", err)
	}
	ctx := context.Background()
	var payload any
	if name == "automation_query" {
		payload, err = service.InspectRuntimeCommand(ctx, sctx, operation, input)
	} else {
		plan, planErr := service.PlanRuntimeCommand(ctx, sctx, operation, input)
		if planErr != nil {
			err = planErr
		} else {
			if requestID == "" {
				requestID = fmt.Sprintf("test-%s-%d", strings.ReplaceAll(t.Name(), "/", "-"), automationCommandRequestSequence.Add(1))
			}
			var applied *automationdomain.AutomationCommandApplyResult
			applied, err = service.ApplyRuntimeCommand(ctx, sctx, automationdomain.AutomationCommandRequest{
				Action: automationdomain.AutomationCommandActionApply, Operation: operation,
				Input: input, RequestID: requestID,
				ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
			}, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"})
			if applied != nil {
				payload = applied.Data
			}
		}
	}
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, true
	}
	text, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal command result: %v", err)
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
	}, false
}

func automationCommandTestRoute(name string, args map[string]any) (string, map[string]any) {
	routed := make(map[string]any, len(args)+1)
	for key, value := range args {
		routed[key] = value
	}
	operation := ""
	switch name {
	case "create_scheduled_task":
		operation = "create"
	case "find_scheduled_tasks":
		operation = "list"
	case "inspect_scheduled_task":
		operation, _ = routed["view"].(string)
		if operation == "" || operation == "status" {
			operation = "get"
		}
	case "update_scheduled_task":
		operation = "update"
	case "delete_scheduled_task":
		operation = "delete"
	case "get_scheduled_task_report":
		operation = "report"
	case "run_scheduled_task":
		operation = "run"
	case "repair_scheduled_task":
		operation = strings.TrimSpace(fmt.Sprint(routed["action"]))
		if operation == "recover" {
			operation = "update"
			routed["enabled"] = false
			routed["cancel_active_run"] = true
		} else {
			operation = "retry_delivery"
		}
		delete(routed, "action")
	case "automation_query", "automation_update":
		return name, routed
	}
	routed["operation"] = operation
	if operation == "list" || operation == "get" || operation == "runs" || operation == "events" || operation == "report" {
		return "automation_query", routed
	}
	return "automation_update", routed
}

func decodeAutomationCommandJSON[T any](t *testing.T, result map[string]any) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal([]byte(automationCommandText(t, result)), &payload); err != nil {
		t.Fatalf("解析 Automation command JSON 失败: %v", err)
	}
	return payload
}

func automationCommandText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("Automation command 返回 content 异常: %+v", result)
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("Automation command 返回 text 异常: %+v", content[0])
	}
	return text
}

func automationCommandTestOwnerContext(ownerUserID string) context.Context {
	if strings.TrimSpace(ownerUserID) == "" {
		return context.Background()
	}
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "command_test",
	})
}

func assertDeliveredAgentMessage(t *testing.T, workspacePath string, session protocol.Session, expectedText string, label string) {
	t.Helper()
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取%s消息失败: %v", label, err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望%s写入 1 条消息，实际 %d", label, len(messages))
	}
	if textutil.FirstNonEmpty(stringFromMessage(messages[0], "content")) != expectedText {
		t.Fatalf("%s正文不正确: %+v", label, messages[0])
	}
	summary, ok := messages[0]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("%s应挂载 result_summary: %+v", label, messages[0])
	}
	if textutil.FirstNonEmpty(stringFromMessage(summary, "subtype")) != "success" {
		t.Fatalf("%s投递终态不正确: %+v", label, messages[0])
	}
}

func assertRunDeliveredToContext(t *testing.T, ctx context.Context, service *Service, jobID string, expectedDeliveryTo string) automationdomain.ScheduledTaskRun {
	t.Helper()
	deliveredRuns, err := service.ListTaskRuns(ctx, jobID)
	if err != nil || len(deliveredRuns) == 0 {
		t.Fatalf("读取投递 run 失败: runs=%+v err=%v", deliveredRuns, err)
	}
	if deliveredRuns[0].DeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("run delivery_status 未记录投递成功: %+v", deliveredRuns[0])
	}
	if deliveredRuns[0].DeliveryTo != expectedDeliveryTo {
		t.Fatalf("run delivery_to 应记录实际解析后的投递目标，实际 %q", deliveredRuns[0].DeliveryTo)
	}
	if deliveredRuns[0].DeliveryAttempts != 1 || deliveredRuns[0].DeliveredAt == nil {
		t.Fatalf("run 投递观测信息不完整: %+v", deliveredRuns[0])
	}
	return deliveredRuns[0]
}

func (s *Service) deliverJobObservation(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	executionSessionKey string,
	observation automationexec.ExecutionObservation,
) jobDeliveryResult {
	return s.deliverJobObservationToTarget(ctx, job, job.Delivery, executionSessionKey, observation)
}

func (s *Service) runDueOnce() {
	now := s.nowFn()
	s.runDueOnceAt(now)
	deliveryAt, err := s.loadDeliveryRetryDeadline(
		context.Background(),
		now,
	)
	if err == nil && deadlineReached(deliveryAt, now) {
		s.startDeliveryRetryBatch(now)
	}
}
