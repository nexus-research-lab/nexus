package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/service/chat"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "github.com/mattn/go-sqlite3"
)

type fakeChatRunner struct {
	permission    *permissionctx.Context
	resultText    string
	assistantText string

	mu       sync.Mutex
	requests []chatsvc.Request
}

func (f *fakeChatRunner) HandleChat(_ context.Context, request chatsvc.Request) error {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.mu.Unlock()

	go func() {
		time.Sleep(20 * time.Millisecond)
		f.permission.BroadcastEvent(context.Background(), request.SessionKey, protocol.EventMessage{
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
		f.permission.BroadcastEvent(context.Background(), request.SessionKey, protocol.EventMessage{
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
		f.permission.BroadcastEvent(context.Background(), request.SessionKey, protocol.NewRoundStatusEvent(
			request.SessionKey,
			request.RoundID,
			"finished",
			"success",
		))
	}()
	return nil
}

func (f *fakeChatRunner) Requests() []chatsvc.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]chatsvc.Request, len(f.requests))
	copy(result, f.requests)
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
	mu    sync.Mutex
	calls []channels.DeliveryTarget
}

func (f *fakeDeliveryRouter) DeliverText(
	_ context.Context,
	_ string,
	_ string,
	target channels.DeliveryTarget,
) (channels.DeliveryTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, target)
	return target, nil
}

func (f *fakeDeliveryRouter) Calls() []channels.DeliveryTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]channels.DeliveryTarget, len(f.calls))
	copy(result, f.calls)
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

func TestServiceRunTaskNowUpdatesRunLedger(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "日报同步",
		AgentID:     "agent-1",
		Instruction: "整理今天的进展",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{
			Kind:            SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "manual", ""),
		},
		Delivery: DeliveryTarget{Mode: DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != RunStatusRunning {
		t.Fatalf("期望立即返回 running，实际为 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == RunStatusSucceeded
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run 记录，实际 %d", len(runs))
	}
	if runs[0].Status != RunStatusSucceeded {
		t.Fatalf("期望 run 成功，实际 %s", runs[0].Status)
	}

	requests := chat.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望 chat runner 收到 1 次请求，实际 %d", len(requests))
	}
	if requests[0].Content != "整理今天的进展" {
		t.Fatalf("下发指令不正确: %s", requests[0].Content)
	}
}

func TestHeartbeatWakeDispatchesMainSession(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "检查今日待办并汇总异常。",
			},
		},
		nil,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 1800,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	text := "请额外检查告警列表"
	result, err := service.WakeHeartbeat(context.Background(), "agent-1", HeartbeatWakeRequest{
		Mode: WakeModeNow,
		Text: &text,
	})
	if err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	if !result.Scheduled {
		t.Fatalf("期望立即唤醒返回 scheduled=true")
	}

	waitFor(t, 2*time.Second, func() bool {
		status, statusErr := service.GetHeartbeatStatus(context.Background(), "agent-1")
		if statusErr != nil {
			return false
		}
		return status.LastHeartbeatAt != nil && status.LastAckAt != nil
	})

	status, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if status.LastHeartbeatAt == nil || status.LastAckAt == nil {
		t.Fatalf("heartbeat 状态没有正确更新: %+v", status)
	}

	requests := chat.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望主会话收到 1 次 heartbeat 请求，实际 %d", len(requests))
	}
	if requests[0].SessionKey != buildMainSessionKey("agent-1") {
		t.Fatalf("heartbeat 主会话键错误: %s", requests[0].SessionKey)
	}
	if !strings.Contains(requests[0].Content, "检查今日待办并汇总异常") || !strings.Contains(requests[0].Content, text) {
		t.Fatalf("heartbeat 指令没有正确拼装: %s", requests[0].Content)
	}
}

func TestServiceStartRunsDueTask(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	service.nowFn = func() time.Time {
		return time.Now().UTC()
	}

	_, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "定时巡检",
		AgentID:     "agent-1",
		Instruction: "执行自动巡检",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(1),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{
			Kind:            SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "scheduler", ""),
		},
		Delivery: DeliveryTarget{Mode: DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if err = service.Start(context.Background()); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer service.Stop()

	waitFor(t, 3*time.Second, func() bool {
		return len(chat.Requests()) > 0
	})
}

func TestServiceRunTaskNowDeliversToRememberedWebSocketRoute(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{
		permission: permission,
		resultText: "巡检完成：CPU 使用率正常",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "delivery", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Delivery",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("准备目标会话失败: %v", err)
	}
	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}

	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "主动巡检播报",
		AgentID:     "agent-1",
		Instruction: "执行巡检并输出结果",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{
			Kind:            SessionTargetNamed,
			NamedSessionKey: "ops-bot",
		},
		Delivery: DeliveryTarget{Mode: DeliveryModeLast},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == RunStatusSucceeded
	})

	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取投递目标 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatalf("投递目标 session 不存在")
	}
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取投递目标消息失败: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望投递目标写入 1 条 assistant 消息，实际 %d", len(messages))
	}
	if firstNonEmptyString(stringFromMessage(messages[0], "content")) != "巡检完成：CPU 使用率正常" {
		t.Fatalf("投递正文不正确: %+v", messages[0])
	}
	summary, ok := messages[0]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("投递目标应挂载 result_summary: %+v", messages[0])
	}
	if firstNonEmptyString(stringFromMessage(summary, "subtype")) != "success" {
		t.Fatalf("投递终态不正确: %+v", messages[0])
	}
}

func TestHeartbeatWakeSuppressesHeartbeatOKDelivery(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{
		permission: permission,
		resultText: "HEARTBEAT_OK",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	store := workspacestore.NewSessionFileStore(workspacePath)
	targetSessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "heartbeat", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   targetSessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Heartbeat",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("准备 heartbeat 目标会话失败: %v", err)
	}
	if err := router.RememberWebSocketRoute(context.Background(), targetSessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}

	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "仅在异常时提醒。",
			},
		},
		router,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 1800,
		TargetMode:   HeartbeatTargetLast,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}
	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", HeartbeatWakeRequest{Mode: WakeModeNow}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		status, statusErr := service.GetHeartbeatStatus(context.Background(), "agent-1")
		return statusErr == nil && status.LastAckAt != nil
	})

	sessionValue, _, err := store.FindSession([]string{workspacePath}, targetSessionKey)
	if err != nil {
		t.Fatalf("读取 heartbeat session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatalf("heartbeat session 不存在")
	}
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取 heartbeat 目标消息失败: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("HEARTBEAT_OK 不应外发，实际写入了 %d 条消息", len(messages))
	}
}

func TestBuildHeartbeatInstructionUsesTasksAndDeduplicatesWakeText(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "tasks:\n- name: inbox\n  interval: 30m\n  prompt: check inbox\n",
			},
		},
		nil,
	)
	payload, err := json.Marshal(map[string]any{"text": "urgent ping"})
	if err != nil {
		t.Fatalf("构造事件 payload 失败: %v", err)
	}
	instruction, err := service.buildHeartbeatInstruction(
		context.Background(),
		"agent-1",
		[]SystemEvent{
			{
				EventID:    "evt-1",
				EventType:  "heartbeat.wake",
				SourceType: "heartbeat",
				SourceID:   "agent-1",
				Payload:    string(payload),
			},
		},
		[]heartbeatWakeRequest{
			{
				AgentID:    "agent-1",
				SessionKey: buildMainSessionKey("agent-1"),
				WakeMode:   WakeModeNow,
				Text:       "urgent ping",
			},
		},
		[]heartbeatWakeRequest{
			{
				AgentID:    "agent-1",
				SessionKey: buildMainSessionKey("agent-1"),
				WakeMode:   WakeModeNextHeartbeat,
				Text:       "follow up soon",
			},
		},
	)
	if err != nil {
		t.Fatalf("buildHeartbeatInstruction 失败: %v", err)
	}
	if !strings.Contains(instruction, "Heartbeat tasks:") || !strings.Contains(instruction, "check inbox") {
		t.Fatalf("tasks 指令未正确拼装: %s", instruction)
	}
	if strings.Count(instruction, "urgent ping") != 1 {
		t.Fatalf("wake/event 文本未去重: %s", instruction)
	}
	if !strings.Contains(instruction, "follow up soon") {
		t.Fatalf("缺少 next-heartbeat 文本: %s", instruction)
	}
}

func TestBuildHeartbeatInstructionFallsBackToEventTypeWhenTextMissing(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	runtimeCloser := &fakeRuntimeSessionCloser{}
	service.SetRuntimeSessionCloser(runtimeCloser)
	payload, err := json.Marshal(map[string]any{"instruction": "do not read this"})
	if err != nil {
		t.Fatalf("构造事件 payload 失败: %v", err)
	}

	instruction, err := service.buildHeartbeatInstruction(
		context.Background(),
		"agent-1",
		[]SystemEvent{
			{
				EventID:    "evt-1",
				EventType:  "cron.trigger",
				SourceType: "cron",
				SourceID:   "agent-1",
				Payload:    string(payload),
			},
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildHeartbeatInstruction 失败: %v", err)
	}
	if strings.Contains(instruction, "do not read this") {
		t.Fatalf("不应读取 instruction 字段: %s", instruction)
	}
	if !strings.Contains(instruction, "cron.trigger") {
		t.Fatalf("缺少 event_type 回退: %s", instruction)
	}
}

func TestGetHeartbeatStatusDegradesPersistedExplicitTargetMode(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	err := service.repository.UpsertHeartbeatState(
		context.Background(),
		"hb_1",
		HeartbeatConfig{
			AgentID:      "agent-1",
			Enabled:      true,
			EverySeconds: 120,
			TargetMode:   HeartbeatTargetExplicit,
			AckMaxChars:  80,
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("预置 heartbeat 状态失败: %v", err)
	}

	status, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if status.TargetMode != HeartbeatTargetNone {
		t.Fatalf("explicit 应降级为 none，实际 %s", status.TargetMode)
	}
	if status.DeliveryError == nil || *status.DeliveryError != heartbeatExplicitTargetUnsupportedMessage {
		t.Fatalf("delivery_error 不正确: %+v", status.DeliveryError)
	}
}

func TestWakeHeartbeatNextHeartbeatDoesNotDispatchBeforeDueTime(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "tasks:\n- name: check\n  interval: 30m\n  prompt: check status\n",
			},
		},
		nil,
	)
	service.nowFn = func() time.Time {
		return time.Now().UTC()
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer service.Stop()

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 120,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", HeartbeatWakeRequest{
		Mode: WakeModeNextHeartbeat,
		Text: stringRef("follow up later"),
	}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)
	if len(chat.Requests()) != 0 {
		t.Fatalf("next-heartbeat 不应提前触发，实际请求数 %d", len(chat.Requests()))
	}
	status, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if !status.PendingWake {
		t.Fatalf("next-heartbeat 在到期前应保持 pending_wake=true")
	}
}

func TestWakeHeartbeatNowWhileRunningKeepsPendingWakeAndQueuedRequest(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	service.mu.Lock()
	state := service.heartbeatState["agent-1"]
	if state == nil {
		service.mu.Unlock()
		t.Fatalf("heartbeat state 不存在")
	}
	state.Running = true
	state.PendingWake = false
	service.mu.Unlock()

	result, err := service.WakeHeartbeat(context.Background(), "agent-1", HeartbeatWakeRequest{
		Mode: WakeModeNow,
	})
	if err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	if !result.Scheduled {
		t.Fatalf("wake-now 应返回 scheduled=true")
	}

	status, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if !status.PendingWake {
		t.Fatalf("running 状态下 wake-now 应保留 pending_wake=true")
	}
	items := service.wakeRequests[buildMainSessionKey("agent-1")]
	if len(items) != 1 || items[0].WakeMode != WakeModeNow {
		t.Fatalf("running 状态下 wake-now 应继续排队等待下一轮消费: %+v", items)
	}
}

func TestRunTaskNowForMainTargetEnqueuesCronTextPayload(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 3600,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "Main payload",
		AgentID:     "agent-1",
		Instruction: "follow up in main session",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{
			Kind:     SessionTargetMain,
			WakeMode: WakeModeNow,
		},
		Delivery: DeliveryTarget{Mode: DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}

	var rawPayload string
	row := db.QueryRow(`SELECT payload FROM automation_system_events WHERE event_type='cron.trigger' ORDER BY created_at DESC, event_id DESC LIMIT 1`)
	if err = row.Scan(&rawPayload); err != nil {
		t.Fatalf("读取 cron.trigger payload 失败: %v", err)
	}
	payload := map[string]any{}
	if err = json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		t.Fatalf("解析 cron.trigger payload 失败: %v", err)
	}
	if strings.TrimSpace(anyString(payload["text"])) != "follow up in main session" {
		t.Fatalf("cron.trigger payload.text 不正确: %v", payload)
	}
	if _, exists := payload["instruction"]; exists {
		t.Fatalf("cron.trigger 不应写 instruction 字段: %v", payload)
	}
}

func TestHeartbeatStatusRunningReflectsHeartbeatExecutionState(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeChatRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	statusBeforeStart, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if statusBeforeStart.Running {
		t.Fatalf("Start 前 running 应为 false")
	}

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer service.Stop()

	statusAfterStart, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if statusAfterStart.Running {
		t.Fatalf("仅启动调度器时 running 不应为 true")
	}

	service.mu.Lock()
	state := service.heartbeatState["agent-1"]
	if state == nil {
		service.mu.Unlock()
		t.Fatalf("heartbeat state 不存在")
	}
	state.Running = true
	service.mu.Unlock()

	statusWhileRunning, err := service.GetHeartbeatStatus(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("GetHeartbeatStatus 失败: %v", err)
	}
	if !statusWhileRunning.Running {
		t.Fatalf("heartbeat 执行中 running 应为 true")
	}
}

func TestDeleteTaskCleansIsolatedAutomationSessions(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		&fakeChatRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	runtimeCloser := &fakeRuntimeSessionCloser{}
	service.SetRuntimeSessionCloser(runtimeCloser)
	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "cleanup-target",
		AgentID:     "agent-1",
		Instruction: "cleanup",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{Kind: SessionTargetIsolated},
		Delivery:      DeliveryTarget{Mode: DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	store := workspacestore.NewSessionFileStore(workspacePath)
	now := time.Now().UTC()
	matchingA := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":run-a", "")
	matchingB := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":run-b", "")
	unrelated := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "keep", "")
	for _, sessionKey := range []string{matchingA, matchingB, unrelated} {
		if _, upsertErr := store.UpsertSession(workspacePath, protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      "agent-1",
			ChannelType:  "automation",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "session",
			Options:      map[string]any{},
			IsActive:     true,
		}); upsertErr != nil {
			t.Fatalf("准备测试会话失败: %v", upsertErr)
		}
	}

	if err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	paths := []string{workspacePath}
	for _, removedKey := range []string{matchingA, matchingB} {
		item, _, findErr := store.FindSession(paths, removedKey)
		if findErr != nil {
			t.Fatalf("查询会话失败: %v", findErr)
		}
		if item != nil {
			t.Fatalf("期望会话被清理: %s", removedKey)
		}
	}
	closed := runtimeCloser.Calls()
	if len(closed) != 2 {
		t.Fatalf("期望关闭 2 个 isolated 会话，实际 %d", len(closed))
	}
	item, _, findErr := store.FindSession(paths, unrelated)
	if findErr != nil {
		t.Fatalf("查询保留会话失败: %v", findErr)
	}
	if item == nil {
		t.Fatalf("不应删除非 automation 会话")
	}
}

func TestRunTaskNowMarksMainEventFailedWhenWakeValidationFails(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeChatRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "main-wake-fail",
		AgentID:     "agent-1",
		Instruction: "wake failed",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{Kind: SessionTargetMain, WakeMode: WakeModeNow},
		Delivery:      DeliveryTarget{Mode: DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, execErr := db.Exec(`UPDATE automation_cron_jobs SET wake_mode='bad-mode' WHERE job_id=?`, task.JobID); execErr != nil {
		t.Fatalf("写入坏 wake_mode 失败: %v", execErr)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err == nil {
		t.Fatalf("期望 RunTaskNow 失败")
	}

	var status string
	row := db.QueryRow(
		`SELECT status FROM automation_system_events WHERE event_type='cron.trigger' ORDER BY created_at DESC, event_id DESC LIMIT 1`,
	)
	if scanErr := row.Scan(&status); scanErr != nil {
		t.Fatalf("读取 system event 状态失败: %v", scanErr)
	}
	if strings.TrimSpace(status) != "failed" {
		t.Fatalf("wake 失败后 event 应标记 failed，实际 %s", status)
	}
}

func TestRunTaskNowSkipsDuplicateExplicitDeliveryWhenTargetMatchesExecutionSession(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeChatRunner{permission: permission, resultText: "done"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "existing-room", "")
	task, err := service.CreateTask(context.Background(), CreateJobInput{
		Name:        "dup-delivery",
		AgentID:     "agent-1",
		Instruction: "run once",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: SessionTarget{
			Kind:            SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: DeliveryTarget{
			Mode:    DeliveryModeExplicit,
			Channel: "websocket",
			To:      sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) > 0 && items[0].Status == RunStatusSucceeded
	})

	if len(delivery.Calls()) != 0 {
		t.Fatalf("execution 会话与显式回传目标一致时不应重复投递")
	}
}

func TestWakeRequestBookkeepingPreservesQueuedRequests(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := buildMainSessionKey("agent-1")
	first := "first"
	second := "second"
	service.recordWakeRequest("agent-1", sessionKey, WakeModeNow, &first)
	service.recordWakeRequest("agent-1", sessionKey, WakeModeNow, &second)
	service.recordWakeRequest("agent-1", sessionKey, WakeModeNextHeartbeat, nil)

	items := service.wakeRequests[sessionKey]
	if len(items) != 3 {
		t.Fatalf("同 session 的 wake request 不应被覆盖，实际条目 %d", len(items))
	}
	if items[0].Text != "first" || items[1].Text != "second" || items[2].WakeMode != WakeModeNextHeartbeat {
		t.Fatalf("wake request 应按到达顺序保留: %+v", items)
	}
}

func TestTakeWakeRequestsKeepsRequestsArrivingAfterDispatchStarts(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := buildMainSessionKey("agent-1")
	first := "first"
	service.recordWakeRequest("agent-1", sessionKey, WakeModeNow, &first)

	immediate, deferred := service.takeWakeRequests("agent-1", sessionKey)
	if len(immediate) != 1 || len(deferred) != 0 {
		t.Fatalf("首次消费 wake request 结果不正确: immediate=%+v deferred=%+v", immediate, deferred)
	}

	second := "second"
	service.recordWakeRequest("agent-1", sessionKey, WakeModeNow, &second)
	remaining := service.wakeRequests[sessionKey]
	if len(remaining) != 1 || remaining[0].Text != "second" {
		t.Fatalf("dispatch 开始后新增的 wake request 应保留到下一轮: %+v", remaining)
	}
}

func TestHeartbeatDispatchClaimsEventsByPayloadAgentID(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	chat := &fakeChatRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		chat,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "tasks:\n- name: check\n  interval: 30m\n  prompt: keep alive\n",
			},
		},
		nil,
	)
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}
	if err := service.repository.InsertSystemEvent(
		context.Background(),
		"evt_payload_agent",
		"cron.trigger",
		"cron",
		"job-unknown",
		map[string]any{
			"agent_id": "agent-1",
			"text":     "payload owned event",
		},
	); err != nil {
		t.Fatalf("预置 system event 失败: %v", err)
	}
	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", HeartbeatWakeRequest{
		Mode: WakeModeNow,
	}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		return len(chat.Requests()) > 0
	})
	requests := chat.Requests()
	if len(requests) == 0 {
		t.Fatalf("未触发 heartbeat 下发")
	}
	if !strings.Contains(requests[0].Content, "payload owned event") {
		t.Fatalf("未消费 payload.agent_id 归属事件: %s", requests[0].Content)
	}
}

func newAutomationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
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
    name VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    source_kind VARCHAR(32) NOT NULL DEFAULT 'manual',
    source_creator_agent_id VARCHAR(64),
    source_context_type VARCHAR(64),
    source_context_id VARCHAR(64),
    source_context_label VARCHAR(255),
    source_session_key VARCHAR(255),
    source_session_label VARCHAR(255),
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE automation_cron_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
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
