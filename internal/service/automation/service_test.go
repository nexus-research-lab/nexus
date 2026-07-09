package automation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestHeartbeatWakeDispatchesMainSession(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "检查今日待办并汇总异常。",
			},
		},
		nil,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 1800,
		TargetMode:   automationdomain.HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	text := "请额外检查告警列表"
	result, err := service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{
		Mode: automationdomain.WakeModeNow,
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

	requests := dm.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望主会话收到 1 次 heartbeat 请求，实际 %d", len(requests))
	}
	if requests[0].SessionKey != automationexec.BuildMainSessionKey("agent-1") {
		t.Fatalf("heartbeat 主会话键错误: %s", requests[0].SessionKey)
	}
	if !strings.Contains(requests[0].Content, "检查今日待办并汇总异常") || !strings.Contains(requests[0].Content, text) {
		t.Fatalf("heartbeat 指令没有正确拼装: %s", requests[0].Content)
	}
}

func TestHeartbeatWakeSuppressesHeartbeatOKDelivery(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
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
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "仅在异常时提醒。",
			},
		},
		router,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 1800,
		TargetMode:   automationdomain.HeartbeatTargetLast,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}
	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{Mode: automationdomain.WakeModeNow}); err != nil {
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
		[]automationdomain.SystemEvent{
			{
				EventID:    "evt-1",
				EventType:  "heartbeat.wake",
				SourceType: "heartbeat",
				SourceID:   "agent-1",
				Payload:    string(payload),
			},
		},
		[]automationexec.HeartbeatWakeRequest{
			{
				AgentID:    "agent-1",
				SessionKey: automationexec.BuildMainSessionKey("agent-1"),
				WakeMode:   automationdomain.WakeModeNow,
				Text:       "urgent ping",
			},
		},
		[]automationexec.HeartbeatWakeRequest{
			{
				AgentID:    "agent-1",
				SessionKey: automationexec.BuildMainSessionKey("agent-1"),
				WakeMode:   automationdomain.WakeModeNextHeartbeat,
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
		[]automationdomain.SystemEvent{
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
		automationdomain.HeartbeatConfig{
			AgentID:      "agent-1",
			Enabled:      true,
			EverySeconds: 120,
			TargetMode:   automationdomain.HeartbeatTargetExplicit,
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
	if status.TargetMode != automationdomain.HeartbeatTargetNone {
		t.Fatalf("explicit 应降级为 none，实际 %s", status.TargetMode)
	}
	if status.DeliveryError == nil || *status.DeliveryError != heartbeatExplicitTargetUnsupportedMessage {
		t.Fatalf("delivery_error 不正确: %+v", status.DeliveryError)
	}
}

func TestWakeHeartbeatNextHeartbeatDoesNotDispatchBeforeDueTime(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
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

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 120,
		TargetMode:   automationdomain.HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{
		Mode: automationdomain.WakeModeNextHeartbeat,
		Text: stringRef("follow up later"),
	}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)
	if len(dm.Requests()) != 0 {
		t.Fatalf("next-heartbeat 不应提前触发，实际请求数 %d", len(dm.Requests()))
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
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   automationdomain.HeartbeatTargetNone,
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

	result, err := service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{
		Mode: automationdomain.WakeModeNow,
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
	items := service.wakeRequests[automationexec.BuildMainSessionKey("agent-1")]
	if len(items) != 1 || items[0].WakeMode != automationdomain.WakeModeNow {
		t.Fatalf("running 状态下 wake-now 应继续排队等待下一轮消费: %+v", items)
	}
}

func TestHeartbeatStatusRunningReflectsHeartbeatExecutionState(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)

	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   automationdomain.HeartbeatTargetNone,
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
	sessionKey := automationexec.BuildMainSessionKey("agent-1")
	first := "first"
	second := "second"
	service.recordWakeRequest("agent-1", sessionKey, automationdomain.WakeModeNow, &first)
	service.recordWakeRequest("agent-1", sessionKey, automationdomain.WakeModeNow, &second)
	service.recordWakeRequest("agent-1", sessionKey, automationdomain.WakeModeNextHeartbeat, nil)

	items := service.wakeRequests[sessionKey]
	if len(items) != 3 {
		t.Fatalf("同 session 的 wake request 不应被覆盖，实际条目 %d", len(items))
	}
	if items[0].Text != "first" || items[1].Text != "second" || items[2].WakeMode != automationdomain.WakeModeNextHeartbeat {
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
	sessionKey := automationexec.BuildMainSessionKey("agent-1")
	first := "first"
	service.recordWakeRequest("agent-1", sessionKey, automationdomain.WakeModeNow, &first)

	immediate, deferred := service.takeWakeRequests("agent-1", sessionKey)
	if len(immediate) != 1 || len(deferred) != 0 {
		t.Fatalf("首次消费 wake request 结果不正确: immediate=%+v deferred=%+v", immediate, deferred)
	}

	second := "second"
	service.recordWakeRequest("agent-1", sessionKey, automationdomain.WakeModeNow, &second)
	remaining := service.wakeRequests[sessionKey]
	if len(remaining) != 1 || remaining[0].Text != "second" {
		t.Fatalf("dispatch 开始后新增的 wake request 应保留到下一轮: %+v", remaining)
	}
}

func TestHeartbeatDispatchClaimsEventsByPayloadAgentID(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{
			files: map[string]string{
				"HEARTBEAT.md": "tasks:\n- name: check\n  interval: 30m\n  prompt: keep alive\n",
			},
		},
		nil,
	)
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 60,
		TargetMode:   automationdomain.HeartbeatTargetNone,
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
	if _, err := service.WakeHeartbeat(context.Background(), "agent-1", automationdomain.HeartbeatWakeInput{
		Mode: automationdomain.WakeModeNow,
	}); err != nil {
		t.Fatalf("WakeHeartbeat 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		return len(dm.Requests()) > 0
	})
	requests := dm.Requests()
	if len(requests) == 0 {
		t.Fatalf("未触发 heartbeat 下发")
	}
	if !strings.Contains(requests[0].Content, "payload owned event") {
		t.Fatalf("未消费 payload.agent_id 归属事件: %s", requests[0].Content)
	}
}
