package dm

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoundRunnerPersistsExternalAssistantReplyReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspacePath := filepath.Join(root, "agent-1")
	history := workspacestore.NewAgentHistoryStore(root)
	dispatcher := &fakeExternalReplyDispatcher{
		result: ExternalReplyResult{
			Channel:                  "telegram",
			To:                       "-1001",
			ThreadID:                 "12",
			PrimaryPlatformMessageID: "42",
			PlatformMessageIDs:       []string{"42", "43"},
		},
	}
	sessionKey := "agent:agent-1:telegram:dm:-1001"
	session := protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "agent-1",
	}
	assistant := protocol.Message{
		"message_id":  "assistant-1",
		"session_key": sessionKey,
		"agent_id":    "agent-1",
		"round_id":    "round-1",
		"role":        "assistant",
		"timestamp":   int64(1000),
		"content": []map[string]any{
			{"type": "text", "text": "已处理。"},
		},
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, assistant); err != nil {
		t.Fatal(err)
	}
	if err := history.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id":  "result-1",
		"session_key": sessionKey,
		"agent_id":    "agent-1",
		"round_id":    "round-1",
		"role":        "result",
		"subtype":     "success",
		"result":      "已处理。",
		"timestamp":   int64(1001),
	}); err != nil {
		t.Fatal(err)
	}
	runner := &roundRunner{
		service:       &Service{replies: dispatcher, Host: runtimehost.Host{History: history}},
		workspacePath: workspacePath,
		session:       session,
		agent:         &protocol.Agent{AgentID: "agent-1"},
		sessionKey:    sessionKey,
		roundID:       "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:     "explicit",
			Channel:  "telegram",
			To:       "-1001",
			ThreadID: "12",
		},
	}

	runner.deliverExternalAssistantReply(context.Background(), assistant)

	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("历史应只保留 assistant 可见消息: %+v", messages)
	}
	delivery, ok := messages[0]["external_delivery"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 未挂载外部投递回执: %+v", messages[0])
	}
	if delivery["channel"] != "telegram" || delivery["target"] != "-1001" || delivery["thread_id"] != "12" {
		t.Fatalf("外部投递目标不正确: %+v", delivery)
	}
	if delivery["primary_platform_message_id"] != "42" {
		t.Fatalf("外部投递主平台 message id 不正确: %+v", delivery)
	}
	ids, ok := delivery["platform_message_ids"].([]string)
	if !ok || len(ids) != 2 || ids[0] != "42" || ids[1] != "43" {
		t.Fatalf("外部投递平台 message ids 不正确: %+v", delivery)
	}
}

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
		service:    &Service{replies: dispatcher},
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
		service:    &Service{replies: dispatcher},
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
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("读取默认 agent 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-title-external-im",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permission)
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

func TestRoundRunnerUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	runner := &roundRunner{
		service:     &Service{Host: runtimehost.Host{Usage: recorder, Runtime: runtimectx.NewManager()}},
		ownerUserID: "user-1",
		sessionKey:  "agent:demo:dm:session",
		roundID:     "round-1",
	}
	result := protocol.Message{
		"role":        "result",
		"message_id":  "result-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 10,
		},
	}
	assistant := protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	}

	runner.recordUsage(result)
	runner.recordTerminalAssistantUsage(assistant)

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望只记录 result 聚合 usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "result-1" {
		t.Fatalf("应记录 result usage，实际=%+v", recorder.inputs[0])
	}
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

	runner.recordUsage(protocol.Message{
		"role":        "result",
		"message_id":  "result-empty",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage":       map[string]any{},
	})
	runner.recordTerminalAssistantUsage(protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "agent:demo:dm:session",
		"round_id":    "round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	})

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望 fallback 记录 assistant usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "assistant-1" {
		t.Fatalf("应 fallback 记录 assistant usage，实际=%+v", recorder.inputs[0])
	}
}

func TestBoundLocalDMForwardsFinalReplyToOriginalIM(t *testing.T) {
	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{service: &Service{replies: dispatcher}, agent: &protocol.Agent{AgentID: "amy"}, sessionKey: "agent:amy:ws:dm:existing", externalReplyTarget: &ExternalReplyTarget{PairingID: "pair", BindingVersion: 2, Channel: "weixin-personal", To: "person", SessionKey: "agent:amy:weixin-personal:dm:person"}}
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
