package dm

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoundRunnerDeliversExternalAssistantReply(t *testing.T) {
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

	runner.deliverExternalAssistantReply(context.Background(), protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "你好，我是五子棋。"},
		},
	})

	calls := dispatcher.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("期望外部回复投递 1 次，实际 %d", len(calls))
	}
	if calls[0].agentID != "agent-1" || calls[0].text != "你好，我是五子棋。" {
		t.Fatalf("外部回复内容不正确: %+v", calls[0])
	}
	if calls[0].target.Channel != "weixin-personal" ||
		calls[0].target.To != "user-1" ||
		calls[0].target.ThreadID != "context-token-1" {
		t.Fatalf("外部回复目标不正确: %+v", calls[0].target)
	}
}

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
		service:       &Service{replies: dispatcher, history: history},
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

func TestRoundRunnerSkipsExternalReplyForWebSocketSession(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeExternalReplyDispatcher{}
	runner := &roundRunner{
		service:    &Service{replies: dispatcher},
		agent:      &protocol.Agent{AgentID: "agent-1"},
		sessionKey: "agent:agent-1:ws:dm:user-1",
		roundID:    "round-1",
		externalReplyTarget: &ExternalReplyTarget{
			Mode:    "explicit",
			Channel: "weixin-personal",
			To:      "user-1",
		},
	}

	runner.deliverExternalAssistantReply(context.Background(), protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "普通网页会话不应发外部通道。"},
		},
	})

	if calls := dispatcher.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("WebSocket 会话不应触发外部回复: %+v", calls)
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

	stop := runner.startExternalReplyTyping(context.Background())
	stop()
	time.Sleep(externalTypingStartDelay + 50*time.Millisecond)

	if calls := dispatcher.typingCallsSnapshot(); len(calls) != 0 {
		t.Fatalf("快速结束不应发送 typing 状态: %+v", calls)
	}
}

func TestScheduleTitleGenerationSkipsRoomConversationForExternalDMSession(t *testing.T) {
	t.Parallel()

	titleScheduler := &fakeDMTitleScheduler{}
	service := &Service{titles: titleScheduler}
	sessionKey := "agent:agent-1:weixin-personal:dm:wx-user-1"
	roomID := "room-agent-1"
	conversationID := "wx-user-1"
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     "owner-a",
		Username:   "owner-a",
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
	service.scheduleTitleGeneration(
		ctx,
		protocol.ParseSessionKey(sessionKey),
		protocol.Session{
			SessionKey:     sessionKey,
			AgentID:        "agent-1",
			ChannelType:    "weixin-personal",
			ChatType:       "dm",
			Title:          "New Chat",
			MessageCount:   1,
			RoomID:         &roomID,
			ConversationID: &conversationID,
		},
		"你好",
		1,
		"kimi-code",
		"kimi-for-coding",
	)

	request := titleScheduler.LastRequest()
	if request.SessionKey != sessionKey {
		t.Fatalf("标题请求 session_key 不正确: %+v", request)
	}
	if request.OwnerUserID != "owner-a" {
		t.Fatalf("标题请求 owner 不正确: %+v", request)
	}
	if request.ConversationID != "" || request.ConversationRoomID != "" || request.ConversationMessageCount != -1 {
		t.Fatalf("外部 DM 不应作为 room conversation 调度标题: %+v", request)
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
	if _, err = service.files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
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
		ReqID:      "round-external-im-title",
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

func TestRefreshSessionMetaPreservesGeneratedTitle(t *testing.T) {
	cfg := newDMTestConfig(t)
	service := NewService(cfg, nil, nil, permissionctx.NewContext())
	workspacePath := filepath.Join(cfg.WorkspacePath, "agent-title")
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-title",
		protocol.SessionChannelWeixinPersonalSegment,
		protocol.RoomTypeDM,
		"wx-user-1",
		"",
	)
	now := time.Now().UTC()
	stale := protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-title",
		ChannelType:  protocol.SessionChannelWeixinPersonal,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    now.Add(-time.Hour),
		LastActivity: now.Add(-time.Minute),
		Title:        "New Chat",
		MessageCount: 75,
		Options:      map[string]any{},
	}
	if _, err := service.files.UpsertSession(workspacePath, stale); err != nil {
		t.Fatalf("写入初始 session 失败: %v", err)
	}
	persisted := stale
	persisted.Title = "午餐建议"
	if _, err := service.files.UpsertSession(workspacePath, persisted); err != nil {
		t.Fatalf("写入生成标题失败: %v", err)
	}

	updated, err := service.refreshSessionMetaRuntimeState(workspacePath, stale)
	if err != nil {
		t.Fatalf("刷新运行态失败: %v", err)
	}
	if updated == nil || updated.Title != "午餐建议" {
		t.Fatalf("运行态刷新不应覆盖已生成标题: %+v", updated)
	}

	updated, err = service.refreshSessionMetaAfterMessage(workspacePath, stale, protocol.Message{
		"message_id":  "assistant-1",
		"role":        "assistant",
		"session_key": sessionKey,
	})
	if err != nil {
		t.Fatalf("刷新消息 meta 失败: %v", err)
	}
	if updated == nil || updated.Title != "午餐建议" {
		t.Fatalf("消息 meta 刷新不应覆盖已生成标题: %+v", updated)
	}
}

func TestRoundRunnerUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	runner := &roundRunner{
		service:     &Service{usage: recorder},
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
		service:     &Service{usage: recorder},
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
