// round_runner_test.go 覆盖 DM 轮次执行器、上下文注入、连接器通告、子代理任务与唤醒测试。
package dm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoundRunnerDiscardsUncommittedDeferredRuntimeMessages(t *testing.T) {
	t.Parallel()

	client := &fakeDMClient{}
	runner := &roundRunner{
		client:            client,
		userMessageID:     "echo-user-1",
		internal:          true,
		deferredAssistant: &DeferredAssistantHooks{},
	}
	if got := runner.runtimeInputOptions().MessageUUID; got != "echo-user-1" {
		t.Fatalf("deferred runtime message uuid = %q, want echo-user-1", got)
	}
	runner.observeDeferredRuntimeMessage(sdkprotocol.ReceivedMessage{UUID: "echo-assistant-1"})
	runner.observeDeferredRuntimeMessage(sdkprotocol.ReceivedMessage{UUID: "echo-assistant-2"})
	if err := runner.discardDeferredRuntimeMessages(); err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.removeMessages) != 1 {
		t.Fatalf("remove messages calls = %#v, want one call", client.removeMessages)
	}
	got := strings.Join(client.removeMessages[0], ",")
	if want := "echo-user-1,echo-assistant-1,echo-assistant-2"; got != want {
		t.Fatalf("removed runtime messages = %q, want %q", got, want)
	}
}

func TestRoundRunnerMaintainsExternalTypingState(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}
	accelerateDMExternalTyping(runner)

	stop := runner.startExternalReplyTyping(context.Background())
	deadline := time.After(2 * time.Second)
	for {
		if calls := dispatcher.typingCallsSnapshot(); len(calls) > 0 && calls[0].active {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("未发送 typing start: %+v", dispatcher.typingCallsSnapshot())
		case <-time.After(10 * time.Millisecond):
		}
	}
	stop()

	calls := dispatcher.typingCallsSnapshot()
	if len(calls) < 2 {
		t.Fatalf("typing start/stop 调用不足: %+v", calls)
	}
	if !calls[0].active || calls[len(calls)-1].active {
		t.Fatalf("typing 状态顺序不正确: %+v", calls)
	}
	if calls[0].target.ThreadID != "context-token-1" || calls[len(calls)-1].target.To != "user-1" {
		t.Fatalf("typing 目标不正确: %+v", calls)
	}
}

func TestRoundRunnerSkipsExternalTypingForQuickReply(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:weixin-personal:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "weixin-personal",
			To:       "user-1",
			ThreadID: "context-token-1",
		},
	}
	accelerateDMExternalTyping(runner)

	stop := runner.startExternalReplyTyping(context.Background())
	stop()
	time.Sleep(runner.externalTypingStartDelay() + 50*time.Millisecond)

	if calls := dispatcher.typingCallsSnapshot(); len(calls) != 0 {
		t.Fatalf("快速结束不应发送 typing 状态: %+v", calls)
	}
}

func TestHandleChatSchedulesTitleForExistingExternalIMDefaultTitle(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		client.sendResultAsync(client.sessionID, "result-title-external-im", "success", "ok")
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	titleScheduler := &fakeDMTitleScheduler{}
	service.SetTitleGenerator(titleScheduler)

	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonalSegment,
		protocol.RoomTypeDM,
		"wx-user-1",
		"",
	)
	now := time.Now().UTC()
	if _, err = service.Files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.SessionChannelWeixinPersonal,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    now.Add(-time.Hour),
		LastActivity: now.Add(-time.Minute),
		Title:        "New Chat",
		MessageCount: 74,
		Options: map[string]any{
			protocol.OptionRuntimeProvider: "kimi-code",
			protocol.OptionRuntimeModel:    "kimi-for-coding",
		},
	}); err != nil {
		t.Fatalf("写入外部 IM session 失败: %v", err)
	}
	sender := newDMTestSender("sender-external-im-title")
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err = service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "中午吃点啥好你觉得",
		RoundID:    "round-external-im-title",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	request := titleScheduler.LastRequest()
	if request.SessionKey != sessionKey {
		t.Fatalf("未为外部 IM session 调度标题生成: %+v", request)
	}
	if request.SessionTitle != "New Chat" || request.SessionMessageCount != 74 {
		t.Fatalf("标题请求未携带默认标题和原始消息数: %+v", request)
	}
	if request.ConversationID != "" || request.ConversationRoomID != "" || request.ConversationMessageCount != -1 {
		t.Fatalf("外部 IM 标题生成不应走 room conversation: %+v", request)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
}

func TestRoundRunnerUsageFallsBackToTerminalAssistantWhenResultUsageEmpty(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	runner := &roundRunner{
		service:     &Service{Host: runtimehost.Host{Usage: recorder, Runtime: runtimectx.NewManager()}},
		ownerUserID: "user-1",
		sessionKey:  "agent:demo:dm:session",
		roundID:     "round-1",
	}

	runner.RecordResultUsage(protocol.Message{
		"role":        "result",
		"message_id":  "result-empty",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage":       map[string]any{},
	}, runner.writeUsage)
	runner.RecordTerminalAssistantUsage(protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	}, runner.writeUsage)

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望 fallback 记录 assistant usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "assistant-1" {
		t.Fatalf("应 fallback 记录 assistant usage，实际=%+v", recorder.inputs[0])
	}
}

func TestBoundLocalDMForwardsFinalReplyToOriginalIM(t *testing.T) {
	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{service: &Service{replies: dispatcher, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}}, agent: &protocol.Agent{AgentID: "amy"}, sessionKey: "agent:amy:ws:dm:existing", externalReplyTarget: &ExternalReplyTarget{PairingID: "pair", BindingVersion: 2, Channel: "weixin-personal", To: "person", SessionKey: "agent:amy:weixin-personal:dm:person"}}
	runner.deliverExternalAssistantReply(t.Context(), protocol.Message{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "原会话的回答"}}})
	calls := dispatcher.callsSnapshot()
	if len(calls) != 1 || calls[0].target.PairingID != "pair" || calls[0].target.BindingVersion != 2 {
		t.Fatalf("reply=%+v", calls)
	}
	runner.externalReplyTarget.PairingID = ""
	runner.deliverExternalAssistantReply(t.Context(), protocol.Message{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "未绑定"}}})
	if len(dispatcher.callsSnapshot()) != 1 {
		t.Fatal("unbound local session forwarded")
	}
}

// DM 告知与 runtimehost 的 compact 清理必须落在同一个会话键上，否则 compact 后状态永远不再告知。
func TestConnectorContextReannouncedAfterCompact(t *testing.T) {
	service := &Service{}
	service.Announcements = &runtimehost.ContextAnnouncements{}
	sessionID := "sdk-1"
	runner := &roundRunner{
		service:          service,
		sessionKey:       "agent:main:dm:1",
		session:          protocol.Session{SessionID: &sessionID},
		connectorContext: "<connector_runtime_state/>",
	}
	if len(runner.connectorContextualInputs()) != 1 {
		t.Fatal("first round must announce connector state")
	}
	if len(runner.connectorContextualInputs()) != 0 {
		t.Fatal("unchanged connector state must not be resent")
	}
	service.Announcements.ForgetAfterCompact(runner.sessionKey, sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{Subtype: "compact_boundary"},
	})
	if len(runner.connectorContextualInputs()) != 1 {
		t.Fatal("compact boundary must force connector state to be announced again")
	}
}

// 恢复上下文改读历史尾页后，结果必须与读取完整历史时一致。
func TestRecoveryContextualInputsMatchFullHistory(t *testing.T) {
	cases := map[string]func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64){
		"failed previous round": func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64) {
			for i := range 12 {
				appendRecoveryRound(t, h, ws, key, fmt.Sprintf("round-%d", i), "success", ts)
			}
			appendRecoveryRound(t, h, ws, key, "round-failed", "error", ts)
		},
		"automation deliveries after last user": func(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, ts *int64) {
			appendRecoveryRound(t, h, ws, key, "round-user", "success", ts)
			for i := range 7 {
				appendAutomationDeliveryRound(t, h, ws, key, i, ts)
			}
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			workspacePath := filepath.Join(root, "agent-1")
			history := workspacestore.NewAgentHistoryStore(root)
			sessionKey := "agent:agent-1:web:dm:recovery"
			session := protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"}
			timestamp := int64(1000)
			seed(t, history, workspacePath, sessionKey, &timestamp)
			if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-current", "now", timestamp+10); err != nil {
				t.Fatal(err)
			}

			full, err := history.ReadMessages(workspacePath, session, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := append(
				conversationsvc.AutomationDeliveryContextualInputs(full, "round-current"),
				conversationsvc.RoundRecoveryContextualInputs(full, "", "round-current")...,
			)
			if len(want) == 0 {
				t.Fatal("fixture 应产生恢复上下文")
			}
			execution := &dmChatExecution{
				service: &Service{Host: runtimehost.Host{History: history}},
				ctx:     context.Background(),
				request: Request{RoundID: "round-current"},
				agent:   &protocol.Agent{AgentID: "agent-1", WorkspacePath: workspacePath},
				session: session,
			}
			if got := execution.recoveryContextualInputs(); !reflect.DeepEqual(got, want) {
				t.Fatalf("恢复上下文与完整历史不一致: got=%d %v want=%d %v", len(got), blockNames(got), len(want), blockNames(want))
			}
		})
	}
}

func appendRecoveryRound(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key, roundID, subtype string, ts *int64) {
	t.Helper()
	*ts += 10
	if err := h.AppendRoundMarker(ws, key, roundID, "question "+roundID, *ts); err != nil {
		t.Fatal(err)
	}
	appendRecoveryOverlay(t, h, ws, key, protocol.Message{
		"message_id": "assistant-" + roundID, "session_key": key, "agent_id": "agent-1", "round_id": roundID,
		"role": "assistant", "timestamp": *ts + 1, "is_complete": subtype == "success",
		"content": []map[string]any{{"type": "text", "text": "answer"}},
	})
	result := protocol.NewHostResultMessage("result-"+roundID, key, "agent-1", roundID, subtype, "done", subtype != "success")
	result["timestamp"] = *ts + 2
	appendRecoveryOverlay(t, h, ws, key, result)
}

func appendAutomationDeliveryRound(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, index int, ts *int64) {
	t.Helper()
	*ts += 10
	roundID := fmt.Sprintf("automation_delivery_round_%d", index)
	if err := h.AppendRoundMarkerWithOptions(ws, key, roundID, "deliver", *ts, workspacestore.RoundMarkerOptions{
		HiddenFromUser: true, Synthetic: true, Purpose: "automation_delivery",
	}); err != nil {
		t.Fatal(err)
	}
	metadata := map[string]any{"source": "automation_delivery", "job_id": "job", "run_id": fmt.Sprintf("run-%d", index)}
	appendRecoveryOverlay(t, h, ws, key, protocol.Message{
		"message_id": fmt.Sprintf("automation_delivery_assistant_%d", index), "session_key": key, "agent_id": "agent-1",
		"round_id": roundID, "role": "assistant", "timestamp": *ts + 1, "stop_reason": "end_turn", "is_complete": true,
		"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("report %d", index)}},
		"metadata": metadata,
	})
	result := protocol.NewHostResultMessage(fmt.Sprintf("automation_delivery_result_%d", index), key, "agent-1", roundID, "success", "report", false)
	result["timestamp"] = *ts + 2
	result["metadata"] = metadata
	appendRecoveryOverlay(t, h, ws, key, result)
}

func appendRecoveryOverlay(t *testing.T, h *workspacestore.AgentHistoryStore, ws, key string, message protocol.Message) {
	t.Helper()
	if err := h.AppendOverlayMessage(ws, key, message); err != nil {
		t.Fatal(err)
	}
}

func blockNames(blocks []runtimectx.ContextualInputBlock) []string {
	names := make([]string, 0, len(blocks))
	for _, block := range blocks {
		names = append(names, block.Name+":"+fmt.Sprint(block.Metadata))
	}
	return names
}

func dmSubagentTaskMessage(subtype string, status string) protocol.Message {
	return protocol.Message{
		"metadata": map[string]any{
			"subtype":    subtype,
			"task_id":    "task-1",
			"agent_id":   "subagent-1",
			"agent_type": "worker",
			"status":     status,
		},
	}
}

// 唤醒说明来自最近一次顶层后台任务通知，交接后清空，避免下一次唤醒复用旧摘要。
func TestRuntimeWakeTriggerUsesLatestTaskNotification(t *testing.T) {
	runner := &roundRunner{}
	runner.observeIdleTaskNotification(sdkprotocol.ReceivedMessage{
		Type:             sdkprotocol.MessageTypeSystem,
		TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b1", Status: "completed", Summary: "first"},
	})
	runner.observeIdleTaskNotification(sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b2", Status: "failed", Summary: "second"}},
	})
	trigger := runner.idleWakeTrigger
	if trigger.TaskID != "b2" || trigger.Content() != "second" {
		t.Fatalf("trigger = %+v", trigger)
	}
	metadata := trigger.Metadata()
	if metadata["input_kind"] != "runtime_wake" || metadata["task_status"] != "failed" || metadata["trigger_kind"] != "background_task" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

// 交接期间普通输入让路并在交接结束后放行；自唤醒 round 自身不受闸门影响。
func TestRuntimeWakeGateYieldsOrdinaryInputUntilHandoffEnds(t *testing.T) {
	service := &Service{}
	release := service.RuntimeWakeGates.Begin("agent:a:ws:dm:c")
	ordinary := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c"}
	err := ordinary.yieldToRuntimeWake()
	var pending *runtimeWakePendingError
	if !errors.As(err, &pending) || !errors.Is(err, runtimehost.ErrRuntimeWakeHandoffPending) {
		t.Fatalf("ordinary yield err = %v", err)
	}
	wake := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c", request: Request{runtimeWake: &runtimeWakeStart{}}}
	if err := wake.yieldToRuntimeWake(); err != nil {
		t.Fatalf("wake round must pass its own gate: %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		release()
		release()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pending.wait(ctx); err != nil {
		t.Fatalf("wait err = %v", err)
	}
	if err := ordinary.yieldToRuntimeWake(); err != nil {
		t.Fatalf("after release err = %v", err)
	}
}
