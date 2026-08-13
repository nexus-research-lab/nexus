package automation

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

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
						"text": firstNonEmptyString(f.assistantText, f.resultText, "ok"),
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
				"result":     firstNonEmptyString(f.resultText, "ok"),
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
	message := firstNonEmptyString(
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
		Room: protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
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
						"text": firstNonEmptyString(f.resultText, "room ok"),
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
				"result":     firstNonEmptyString(f.resultText, "room ok"),
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
	_ string,
	text string,
	target channels.DeliveryTarget,
) (channels.DeliveryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, target)
	f.messages = append(f.messages, text)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
	if f.err != nil {
		return channels.DeliveryResult{}, f.err
	}
	return channels.DeliveryResult{Target: target, Receipt: f.receipt}, nil
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

type sessionArtifactDeletionCall struct {
	ownerUserID      string
	workspacePath    string
	sessionKey       string
	cleanupSessionID string
}

type fakeSessionArtifactDeletionCoordinator struct {
	mu       sync.Mutex
	calls    []sessionArtifactDeletionCall
	err      error
	deleteFn func(context.Context, sessionArtifactDeletionCall) error
}

func (f *fakeSessionArtifactDeletionCoordinator) DeleteSessionArtifacts(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	sessionKey string,
	cleanupSessionID string,
) error {
	call := sessionArtifactDeletionCall{
		ownerUserID:      ownerUserID,
		workspacePath:    workspacePath,
		sessionKey:       sessionKey,
		cleanupSessionID: cleanupSessionID,
	}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	deleteFn := f.deleteFn
	err := f.err
	f.mu.Unlock()
	if deleteFn != nil {
		if deleteErr := deleteFn(ctx, call); deleteErr != nil {
			return deleteErr
		}
	}
	return err
}

func (f *fakeSessionArtifactDeletionCoordinator) Calls() []sessionArtifactDeletionCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]sessionArtifactDeletionCall, len(f.calls))
	copy(result, f.calls)
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
    id VARCHAR(64) NOT NULL PRIMARY KEY
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
    configuration_version INTEGER NOT NULL DEFAULT 1,
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
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
CREATE TABLE automation_heartbeat_states (
    state_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL,
    every_seconds INTEGER NOT NULL,
    target_mode VARCHAR(32) NOT NULL,
    ack_max_chars INTEGER NOT NULL,
    last_heartbeat_at DATETIME,
    last_ack_at DATETIME,
    configuration_version INTEGER NOT NULL DEFAULT 1,
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
CREATE TABLE automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL DEFAULT '',
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

func stringRef(value string) *string {
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

func firstNonEmptyString(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
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
		if firstNonEmptyString(strings.TrimSpace(messageAnyString(item["type"]))) != "text" {
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
