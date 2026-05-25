package automation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type fakeDMRunner struct {
	permission    *permissionctx.Context
	resultText    string
	assistantText string
	delay         time.Duration
	requiredTool  string

	mu         sync.Mutex
	requests   []dmsvc.Request
	interrupts []dmsvc.InterruptRequest
}

func (f *fakeDMRunner) HandleChat(_ context.Context, request dmsvc.Request) error {
	f.mu.Lock()
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
		if f.emitPermissionDeniedResult(context.Background(), request, emit) {
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
	toolName := strings.TrimSpace(f.requiredTool)
	if toolName == "" || request.PermissionHandler == nil {
		return false
	}
	decision, err := request.PermissionHandler(ctx, sdkpermission.Request{ToolName: toolName})
	if err != nil {
		decision = sdkpermission.Deny(err.Error(), false)
	}
	if decision.Behavior == sdkpermission.BehaviorAllow {
		return false
	}
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
	return true
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

	mu         sync.Mutex
	requests   []roomsvc.ChatRequest
	interrupts []roomsvc.InterruptRequest
	err        error
}

func (f *fakeRoomRunner) HandleChat(_ context.Context, request roomsvc.ChatRequest) error {
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

func (f *fakeRoomRunner) Requests() []roomsvc.ChatRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]roomsvc.ChatRequest, len(f.requests))
	copy(result, f.requests)
	return result
}

func (f *fakeRoomRunner) HandleInterrupt(_ context.Context, request roomsvc.InterruptRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interrupts = append(f.interrupts, request)
	return nil
}

func (f *fakeRoomRunner) Interrupts() []roomsvc.InterruptRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]roomsvc.InterruptRequest, len(f.interrupts))
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
	ownerUserIDs []string
	err          error
}

func (f *fakeDeliveryRouter) DeliverText(
	ctx context.Context,
	_ string,
	_ string,
	target channels.DeliveryTarget,
) (channels.DeliveryTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, target)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
	if f.err != nil {
		return channels.DeliveryTarget{}, f.err
	}
	return target, nil
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

type fakeRuntimeSessionCloser struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeRuntimeSessionCloser) CloseSession(_ context.Context, sessionKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sessionKey)
	return nil
}

func (f *fakeRuntimeSessionCloser) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]string, len(f.calls))
	copy(result, f.calls)
	return result
}

func newAutomationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	schema := `
CREATE TABLE agents (
    id VARCHAR(64) NOT NULL PRIMARY KEY
);
CREATE TABLE automation_cron_jobs (
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
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
    execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent',
    session_target_kind VARCHAR(32) NOT NULL,
    bound_session_key VARCHAR(255),
    named_session_key VARCHAR(255),
    wake_mode VARCHAR(32) NOT NULL,
    delivery_mode VARCHAR(32) NOT NULL,
    delivery_channel VARCHAR(64),
    delivery_to VARCHAR(255),
    delivery_account_id VARCHAR(64),
    delivery_thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    next_run_at DATETIME,
    running_run_id VARCHAR(64),
    running_started_at DATETIME,
    last_run_at DATETIME,
    last_run_status VARCHAR(32),
    failure_streak INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    last_delivery_status VARCHAR(32),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_cron_runs (
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
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
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

type testAgentResolver struct {
	workspacePath string
}

func (r *testAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	return &protocol.Agent{
		AgentID:       agentID,
		WorkspacePath: r.workspacePath,
	}, nil
}

func (r *testAgentResolver) GetDefaultAgent(_ context.Context) (*protocol.Agent, error) {
	return &protocol.Agent{
		AgentID:       "nexus",
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
