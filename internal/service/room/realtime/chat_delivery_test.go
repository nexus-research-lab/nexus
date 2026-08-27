package realtime_test

import (
	"context"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealtimeServiceHandleChatWithSingleAgentRoomFallbackTarget(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-usage",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "单聊助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-sdk-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "已收到，正在处理。"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-sdk-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    15,
					DurationAPIMS: 11,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  3,
						"output_tokens": 5,
					},
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-1")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:      sharedSessionKey,
		RoomID:          roomContext.Room.ID,
		ConversationID:  roomContext.Conversation.ID,
		ClientRequestID: "request-room-user-1",
		ClientMessageID: "local-room-user-1",
		Content:         "你好",
		RoundID:         "room-round-1",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertRoomEventTypes(t, events, []protocol.EventType{
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
		protocol.EventTypeChatAck,
		protocol.EventTypeSessionStatus,
		protocol.EventTypeAgentRoundStatus,
		protocol.EventTypeStreamStart,
		protocol.EventTypeMessage,
		protocol.EventTypeMessage,
		protocol.EventTypeAgentRoundStatus,
		protocol.EventTypeStreamEnd,
		protocol.EventTypeRoundStatus,
	})
	client.mu.Lock()
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryPrompts) != 1 {
		t.Fatalf("期望发送 1 条 Room runtime query，实际 %d", len(queryPrompts))
	}
	for _, expected := range []string{
		"<public_anchor>",
		"<public_feed>",
		"<latest_trigger>",
	} {
		if !strings.Contains(queryPrompts[0], expected) {
			t.Fatalf("Room runtime query 缺少动态上下文 %q:\n%s", expected, queryPrompts[0])
		}
	}
	if strings.Contains(queryPrompts[0], "<nexus_runtime_context>") {
		t.Fatalf("默认关闭情绪系统时不应注入动态上下文:\n%s", queryPrompts[0])
	}

	pendingMsgID := ""
	foundCorrelatedUserEvent := false
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage && event.Data["role"] == "user" {
			foundCorrelatedUserEvent = event.Data["client_message_id"] == "local-room-user-1"
		}
		if event.EventType == protocol.EventTypeChatAck {
			if pending, ok := event.Data["pending"].([]protocol.ChatAckPendingSlot); ok && len(pending) > 0 {
				pendingMsgID = pending[0].MsgID
			}
		}
		if event.EventType == protocol.EventTypeMessage && event.MessageID == "assistant-sdk-1" {
			if pendingMsgID == "" {
				t.Fatal("未拿到 pending slot msg_id")
			}
			if event.MessageID == pendingMsgID {
				t.Fatalf("assistant message_id 不应回退成 slot msg_id: %s", pendingMsgID)
			}
		}
	}
	if !foundCorrelatedUserEvent {
		t.Fatal("Room durable user 广播必须携带 client_message_id 以原位替换 optimistic 消息")
	}

	roomSystemPrompt := factory.LastOptions().System.Append
	roomPromptOptions := factory.LastOptions().System
	if !strings.Contains(roomPromptOptions.AppendStatic, "## Execution Orchestration") ||
		!strings.Contains(roomPromptOptions.AppendStatic, "# Nexus Room") ||
		!strings.Contains(roomPromptOptions.AppendStatic, "<room_member_directory>") {
		t.Fatalf("Room 稳定 prompt 应包含 execution contract、房间规则与成员目录: %q", roomPromptOptions.AppendStatic)
	}
	if strings.Contains(roomPromptOptions.AppendDynamic, "## Execution Orchestration") ||
		strings.Contains(roomPromptOptions.AppendDynamic, "# Nexus Room") ||
		strings.Contains(roomPromptOptions.AppendDynamic, "<room_member_directory>") {
		t.Fatalf("Room 动态 prompt 不应重复房间稳定段: %q", roomPromptOptions.AppendDynamic)
	}
	if strings.Count(roomPromptOptions.AppendStatic, "## Execution Orchestration") != 1 ||
		strings.Count(roomSystemPrompt, "## Execution Orchestration") != 1 {
		t.Fatalf("Room execution contract 应只注入一次: static=%q combined=%q", roomPromptOptions.AppendStatic, roomSystemPrompt)
	}
	for _, expected := range []string{
		"# Nexus Room",
		"You are a member in a multi-member Nexus Room",
		"Each turn includes <public_feed>",
		"Before substantial execution, assess separability",
		"members may use local subagents",
		"Current-Room private messaging is disabled",
		`"room host default takeover"`,
		"managed Plan and assign_work through execution-orchestrator",
		"never substitute raw @",
		"If a private message wakes you, answer once in the final reply",
		"The final reply may be persisted or projected verbatim",
		"# Nexus Room Member Directory",
		"<room_member_directory>",
		"- name=单聊助手 agent_id=" + memberAgent.AgentID,
	} {
		if !strings.Contains(roomSystemPrompt, expected) {
			t.Fatalf("Room 固定规则应注入 SDK append system prompt，缺少 %q:\n%s", expected, roomSystemPrompt)
		}
	}
	for _, unexpected := range []string{
		`nexus.send_message`,
		"以成员 单聊助手",
		"<current_room_member>",
	} {
		if strings.Contains(roomSystemPrompt, unexpected) {
			t.Fatalf("Room 固定规则不应包含动态变量 %q:\n%s", unexpected, roomSystemPrompt)
		}
	}
	if got := strings.TrimSpace(factory.LastOptions().Env["NEXUS_PROJECT_ROOT"]); got != "" {
		t.Fatalf("Room runtime 不应再注入项目根目录: got=%q", got)
	}
	for _, key := range []string{"NEXUSCTL_COMMAND_PATH", "NEXUSCTL_USER_ID", "NEXUSCTL_WORKSPACE_PATH"} {
		if got := strings.TrimSpace(factory.LastOptions().Env[key]); got != "" {
			t.Fatalf("Room runtime 不得获得 owner-scoped CLI 环境 %s=%q: %+v", key, got, factory.LastOptions().Env)
		}
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, memberAgent.AgentID, roomContext.Room.RoomType)
	cursor, ok, err := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ReadRoomPublicCursor(
		memberAgent.WorkspacePath,
		privateSessionKey,
		roomContext.Conversation.ID,
		memberAgent.AgentID,
	)
	if err != nil {
		t.Fatalf("读取 Room 公区 cursor 失败: %v", err)
	}
	if !ok || !strings.HasPrefix(cursor.LastPublicMessageID, "msg_user_") {
		t.Fatalf("成功 round 应记录目标 agent 公区消费位置: ok=%v cursor=%+v", ok, cursor)
	}
	roomTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-user-1",
			"sessionId": client.sessionID,
			"timestamp": roomTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-sdk-1",
			"sessionId":  client.sessionID,
			"parentUuid": "room-user-1",
			"timestamp":  roomTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "已收到，正在处理。"},
				},
			},
		},
	})
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 2 {
		t.Fatalf("共享消息数量不正确: got=%d want=2", len(sharedMessages))
	}
	if _, exists := sharedMessages[0]["client_message_id"]; exists {
		t.Fatalf("client_message_id 不应写入 Room 历史消息: %+v", sharedMessages[0])
	}
	if sharedMessages[1]["message_id"] != "assistant-sdk-1" {
		t.Fatalf("共享 assistant message_id 不正确: %+v", sharedMessages[1])
	}
	sharedSummary, ok := sharedMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToString(sharedSummary["result"]) != "done" {
		t.Fatalf("共享 result 摘要应挂在 assistant 上: %+v", sharedMessages[1])
	}
	privateMessages := readRoomPrivateHistory(
		t,
		cfg.WorkspacePath,
		roomContext.Room.OwnerUserID,
		memberAgent.WorkspacePath,
		privateSessionKey,
		memberAgent.AgentID,
		client.sessionID,
	)
	if len(privateMessages) != 2 {
		t.Fatalf("私有 runtime 消息数量不正确: got=%d want=2", len(privateMessages))
	}
	if privateMessages[0]["role"] != "user" || privateMessages[1]["role"] != "assistant" {
		t.Fatalf("私有 runtime 消息顺序不正确: %+v", privateMessages)
	}
	privateUserContent := anyToString(privateMessages[0]["content"])
	for _, expected := range []string{
		"<public_feed>",
		"User: 你好",
	} {
		if !strings.Contains(privateUserContent, expected) {
			t.Fatalf("私有 round marker 应记录实际 Room dispatch prompt，缺少 %q:\n%s", expected, privateUserContent)
		}
	}
	privateSummary, ok := privateMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToInt(privateSummary["duration_ms"]) != 15 || anyToString(privateSummary["result"]) != "done" {
		t.Fatalf("私有 result 应保留 runtime 摘要: %+v", privateMessages[1])
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-usage")
	if err != nil {
		t.Fatalf("读取 room token usage 失败: %v", err)
	}
	if usageSummary.InputTokens != 3 || usageSummary.OutputTokens != 5 || usageSummary.TotalTokens != 8 {
		t.Fatalf("room result usage 未写入 ledger: %+v", usageSummary)
	}
	if usageSummary.SessionCount != 1 || usageSummary.MessageCount != 1 {
		t.Fatalf("room usage 计数不正确: %+v", usageSummary)
	}
}

func TestRealtimeServiceRoutesUnmentionedGroupMessageToRoomHost(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-host-default",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{amy.AgentID, devin.AgentID},
		Name:                 "群主接管测试房间",
		HostAgentID:          amy.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	hostPrompt := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) error {
		hostPrompt <- prompt
		go sendFakeAssistantResult(client, "amy-room-host-default", "我来处理这条需求。")
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := realtimesvc.NewServiceWithFactory(cfg, roomService, agentService, runtimectx.NewManager(), permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-host-default")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "帮我拆一下这个需求",
		RoundID:        "room-round-host-default",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-host-default" &&
			event.Data["status"] == "finished"
	})
	select {
	case prompt := <-hostPrompt:
		if !strings.Contains(prompt, "room host default takeover") || !strings.Contains(prompt, "帮我拆一下这个需求") {
			t.Fatalf("群主 prompt 缺少默认接管上下文: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("未 @ 消息没有唤醒群主")
	}
	if !hasChatAckPendingAgent(events, amy.AgentID) {
		t.Fatalf("事件流缺少群主 pending slot: %+v", events)
	}
	if hasChatAckPendingAgent(events, devin.AgentID) {
		t.Fatalf("未 @ 消息不应直接唤醒非群主成员: %+v", events)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 公区历史失败: %v", err)
	}
	foundUserMessage := false
	for _, message := range sharedMessages {
		if message["round_id"] == "room-round-host-default" && message["role"] == "user" {
			foundUserMessage = true
			if message["content"] != "帮我拆一下这个需求" {
				t.Fatalf("群主默认接管用户输入内容不正确: %+v", message)
			}
		}
	}
	if !foundUserMessage {
		t.Fatalf("群主默认接管的用户输入应写入公区历史: %+v", sharedMessages)
	}
}

func TestRealtimeServiceAcksPublicMessageWithoutMention(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区无 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-mention")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "先记一下这个背景",
		RoundID:        "room-round-no-mention",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if countEventType(events, protocol.EventTypeChatAck) != 1 {
		t.Fatalf("公区无 @ 消息也必须 ack，否则前端发送队列会卡住: %+v", events)
	}
}

func TestRealtimeServiceSuppressesNoReplyMarkerProjection(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-no-reply",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	agentValue := createTestAgent(t, agentService, ctx, "Amy")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "无需回复测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type": "text_delta",
							"text": "<nexus_room_no_reply/>",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_stop",
						"index": 0,
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "amy-no-reply-result",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					Result:     "<nexus_room_no_reply/>",
					NumTurns:   1,
					DurationMS: 1,
					Usage: map[string]any{
						"input_tokens":  7,
						"output_tokens": 2,
					},
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-reply")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 这条不用你回答",
		RoundID:        "room-round-no-reply",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if hasAgentPublicMessage(events, agentValue.AgentID) {
		t.Fatalf("无需回复标记不应投影到公区: %+v", events)
	}
	if hasStreamText(events, "<nexus_room_no_reply/>") {
		t.Fatalf("无需回复标记不应以流式文本暴露给前端: %+v", events)
	}
	if !hasAgentRoundStatus(events, agentValue.AgentID, "finished") {
		t.Fatalf("no-reply 仍须广播 slot 终态，让前端收口已发布的 thinking 快照: %+v", events)
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-no-reply")
	if err != nil {
		t.Fatalf("读取 no-reply token usage 失败: %v", err)
	}
	if usageSummary.TotalTokens != 9 {
		t.Fatalf("no-reply result usage 也应写入 ledger: %+v", usageSummary)
	}
}

// 活跃目标投递测试。

func TestRealtimeServiceHostConsumesQueuedInputAsSoonAsItsSlotFinishes(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-active-room-target",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	worker := createTestAgent(t, agentService, ctx, "Worker")
	host := createTestAgent(t, agentService, ctx, "Host")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{worker.AgentID, host.AgentID},
		Name:                 "活跃插话目标测试",
		HostAgentID:          host.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	type followUpQuery struct {
		client *fakeRoomClient
		prompt string
	}
	hostFollowUpPrompt := make(chan followUpQuery, 1)
	newReusableClient := func() *fakeRoomClient {
		client := newFakeRoomClient()
		queryCount := 0
		client.onQuery = func(_ context.Context, prompt string) error {
			queryCount++
			if queryCount == 1 {
				return nil
			}
			hostFollowUpPrompt <- followUpQuery{client: client, prompt: prompt}
			go sendFakeAssistantResult(client, "assistant-host-follow-up", "已处理补充要求")
			return nil
		}
		return client
	}
	firstClient := newReusableClient()
	secondClient := newReusableClient()
	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{firstClient, secondClient}}
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		factory,
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-active-target")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Host @Worker 同时处理当前任务",
		RoundID:        "room-round-active-host-worker",
	}); err != nil {
		t.Fatalf("启动 Host/Worker shared round 失败: %v", err)
	}
	started := map[string]bool{}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType == protocol.EventTypeStreamStart {
			started[event.AgentID] = true
		}
		return started[worker.AgentID] && started[host.AgentID]
	})
	options := factory.Options()
	if len(options) != 2 {
		t.Fatalf("shared round runtime 数量 = %d, want 2", len(options))
	}
	hostClient := firstClient
	workerClient := secondClient
	if options[1].CWD == host.WorkspacePath {
		hostClient, workerClient = secondClient, firstClient
	} else if options[0].CWD != host.WorkspacePath {
		t.Fatalf("无法按 workspace 识别 Host runtime: %+v", options)
	}

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "然后重点检查一下边界条件",
		RoundID:        "room-round-unmentioned-host-queue",
		UserMessageID:  "msg-unmentioned-host-queue",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("发送无 @ Host 队列输入失败: %v", err)
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue"
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue" {
			if committed, _ := event.Data["user_message_committed"].(bool); committed {
				t.Fatalf("Host 尚未消费时用户消息不应提交: %+v", event.Data)
			}
		}
	}

	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	hostLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	hostItems, err := queueStore.Snapshot(hostLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(hostItems) != 1 || hostItems[0].SourceMessageID != "msg-unmentioned-host-queue" ||
		len(hostItems[0].TargetAgentIDs) != 1 || hostItems[0].TargetAgentIDs[0] != host.AgentID {
		t.Fatalf("无 @ 输入应只进入 Host 消费队列: %+v", hostItems)
	}
	workerItems, err := queueStore.Snapshot(workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  worker.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, worker.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workerItems) != 0 {
		t.Fatalf("无 @ 输入不应被运行中的 Worker 抢走: %+v", workerItems)
	}

	go sendFakeAssistantResult(hostClient, "assistant-host-current", "Host 当前任务完成")
	select {
	case followUp := <-hostFollowUpPrompt:
		if followUp.client != hostClient {
			t.Fatal("Host 队列输入被其他 Agent runtime 消费")
		}
		if !strings.Contains(followUp.prompt, "然后重点检查一下边界条件") {
			t.Fatalf("Host follow-up prompt 缺少队列输入: %s", followUp.prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host slot 结束后未立即消费队列，仍在等待 Worker")
	}
	if service.CountRunningTasks(worker.AgentID) == 0 {
		t.Fatal("测试前提失效：Host 接力时 Worker 应仍在运行")
	}
	go sendFakeAssistantResult(workerClient, "assistant-worker-current", "Worker 当前任务完成")
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		generating, _ := event.Data["is_generating"].(bool)
		return event.EventType == protocol.EventTypeSessionStatus && !generating
	})
}

func TestRealtimeServiceWakesMentionedAgentFromPublicAssistantReply(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	casey := createTestAgent(t, agentService, ctx, "Casey")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID, casey.AgentID},
		Name:     "公区 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyClient := newFakeRoomClient()
	devinClient := newFakeRoomClient()
	caseyClient := newFakeRoomClient()
	targetPrompts := make(chan string, 2)
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{amyClient, devinClient, caseyClient}}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)

	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(
			amyClient,
			"amy-public-mention-1",
			"@Devin 请查询天气，@Casey 请检查穿衣建议，并分别在公区回复。",
		)
		return nil
	}
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		targetPrompts <- prompt
		go sendFakeAssistantResult(devinClient, "devin-public-mention-1", "天气查询完成。")
		return nil
	}
	caseyClient.onQuery = func(_ context.Context, prompt string) error {
		targetPrompts <- prompt
		go sendFakeAssistantResult(caseyClient, "casey-public-mention-1", "穿衣建议检查完成。")
		return nil
	}

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 和 Casey 分头处理",
		RoundID:        "room-round-public-mention",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	annotatedTargetReplies := make(map[string]struct{}, 2)
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType == protocol.EventTypeMessage &&
			(event.MessageID == "devin-public-mention-1" || event.MessageID == "casey-public-mention-1") &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil {
			annotatedTargetReplies[event.MessageID] = struct{}{}
		}
		generating, _ := event.Data["is_generating"].(bool)
		return len(annotatedTargetReplies) == 2 &&
			event.EventType == protocol.EventTypeSessionStatus && !generating
	})
	var assistantPayload protocol.Message
	assistantEventIndex := -1
	for eventIndex, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != "amy-public-mention-1" || event.Data["role"] != "assistant" {
			continue
		}
		candidate := protocol.Message(event.Data)
		if candidate["is_complete"] == true {
			assistantPayload = candidate
			assistantEventIndex = eventIndex
		}
	}
	if assistantPayload == nil {
		t.Fatalf("未找到最终 assistant 事件: %+v", events)
	}
	mentions, ok := assistantPayload["agent_mentions"].([]protocol.AgentMention)
	if !ok || len(mentions) != 2 {
		t.Fatalf("最终 assistant 事件应带两个可渲染且可交接的 mention: %+v", assistantPayload)
	}
	mentionByAgentID := make(map[string]protocol.AgentMention, len(mentions))
	for _, mention := range mentions {
		if mention.HandoffID == "" {
			t.Fatalf("每个目标 mention 都应带 handoff_id: %+v", mentions)
		}
		mentionByAgentID[mention.AgentID] = mention
	}
	for _, targetAgentID := range []string{devin.AgentID, casey.AgentID} {
		if _, exists := mentionByAgentID[targetAgentID]; !exists {
			t.Fatalf("最终 assistant 事件缺少目标 %s: %+v", targetAgentID, mentions)
		}
	}
	publicWakeSlotByAgentID := make(map[string]protocol.ChatAckPendingSlot, len(mentions))
	publicWakeSlotEventIndexByAgentID := make(map[string]int, len(mentions))
	for eventIndex, event := range events {
		if event.EventType != protocol.EventTypeChatAck {
			continue
		}
		pending, pendingOK := event.Data["pending"].([]protocol.ChatAckPendingSlot)
		if !pendingOK {
			continue
		}
		for _, slot := range pending {
			if _, expected := mentionByAgentID[slot.AgentID]; expected {
				publicWakeSlotByAgentID[slot.AgentID] = slot
				publicWakeSlotEventIndexByAgentID[slot.AgentID] = eventIndex
			}
		}
	}
	for _, targetAgentID := range []string{devin.AgentID, casey.AgentID} {
		publicWakeSlot, foundPublicWakeSlot := publicWakeSlotByAgentID[targetAgentID]
		if !foundPublicWakeSlot {
			t.Fatalf("事件流缺少目标 %s 的公区 @ 唤醒 slot: %+v", targetAgentID, events)
		}
		if publicWakeSlot.HandoffID != mentionByAgentID[targetAgentID].HandoffID {
			t.Fatalf(
				"目标 %s 的 public wake slot handoff_id = %q, want source mention %q",
				targetAgentID,
				publicWakeSlot.HandoffID,
				mentionByAgentID[targetAgentID].HandoffID,
			)
		}
		if publicWakeSlotEventIndex := publicWakeSlotEventIndexByAgentID[targetAgentID]; assistantEventIndex < 0 ||
			assistantEventIndex >= publicWakeSlotEventIndex {
			t.Fatalf(
				"source final message index = %d, target %s public wake slot index = %d; mention 状态必须先可见再由同一 handoff 接棒",
				assistantEventIndex,
				targetAgentID,
				publicWakeSlotEventIndex,
			)
		}
	}
	for range 2 {
		select {
		case prompt := <-targetPrompts:
			if !strings.Contains(prompt, "<latest_trigger>\nAmy: @Devin 请查询天气，@Casey 请检查穿衣建议") {
				t.Fatalf("目标 prompt 缺少完整的多 @ 触发上下文: %s", prompt)
			}
			if strings.Contains(prompt, "type:") || strings.Contains(prompt, "fanout_targets:") {
				t.Fatalf("目标动态 prompt 不应包含字段化 trigger: %s", prompt)
			}
			if strings.Contains(prompt, "<room_member_directory>") {
				t.Fatalf("目标动态 prompt 不应重复成员目录: %s", prompt)
			}
		case <-time.After(time.Second):
			t.Fatal("并非所有被 @ 的目标都收到 runtime 唤醒")
		}
	}
	roomSystemPrompt := factory.LastOptions().System.Append
	if !strings.Contains(roomSystemPrompt, "<room_member_directory>") ||
		!strings.Contains(roomSystemPrompt, "agent_id="+devin.AgentID) ||
		!strings.Contains(roomSystemPrompt, "agent_id="+casey.AgentID) {
		t.Fatalf("目标 system prompt 应包含完整 Room 成员目录: %s", roomSystemPrompt)
	}
	handoffs, err := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath).ListRoot(
		roomContext.Room.OwnerUserID,
		roomContext.Conversation.ID,
		"room-round-public-mention",
	)
	if err != nil {
		t.Fatalf("读取多 @ handoff ledger 失败: %v", err)
	}
	if len(handoffs) != 2 {
		t.Fatalf("多 @ source 应持久化两条独立 handoff: %+v", handoffs)
	}
	handoffByTargetAgentID := make(map[string]workspacestore.RoomPublicHandoff, len(handoffs))
	for _, handoff := range handoffs {
		handoffByTargetAgentID[handoff.TargetAgentID] = handoff
	}
	for _, targetAgentID := range []string{devin.AgentID, casey.AgentID} {
		handoff, exists := handoffByTargetAgentID[targetAgentID]
		if !exists || handoff.HandoffID != mentionByAgentID[targetAgentID].HandoffID {
			t.Fatalf("ledger 缺少目标 %s 对应的独立 handoff: %+v", targetAgentID, handoffs)
		}
		if handoff.Status != "finished" {
			t.Fatalf("目标 %s handoff 未随 runtime 收口: %+v", targetAgentID, handoff)
		}
	}
	targetMessageIDs := map[string]struct{}{
		"devin-public-mention-1": {},
		"casey-public-mention-1": {},
	}
	realtimeReplies := make(map[string]*protocol.PublicHandoffReply, len(targetMessageIDs))
	annotatedReplyEvents := make(map[string]int, len(targetMessageIDs))
	for _, event := range events {
		_, expected := targetMessageIDs[event.MessageID]
		if !expected || event.EventType != protocol.EventTypeMessage ||
			event.Data["role"] != "assistant" || event.Data["is_complete"] != true {
			continue
		}
		targetAgentID := event.AgentID
		reply := protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"])
		if reply == nil {
			continue
		}
		if reply.HandoffID != mentionByAgentID[targetAgentID].HandoffID ||
			reply.SourceMessageID != "amy-public-mention-1" || reply.SourceAgentID != amy.AgentID {
			t.Fatalf("realtime target reply annotation mismatch: target=%s expected_handoff=%q expected_source=%q event=%+v reply=%+v",
				targetAgentID, mentionByAgentID[targetAgentID].HandoffID, amy.AgentID, event, reply)
		}
		if event.Data["agent_mentions"] != nil {
			t.Fatalf("plain target reply must not synthesize reciprocal @: %+v", event.Data)
		}
		realtimeReplies[event.MessageID] = reply
		annotatedReplyEvents[event.MessageID]++
	}
	if len(realtimeReplies) != len(targetMessageIDs) {
		t.Fatalf("not every target realtime final carried handoff_reply: %+v", realtimeReplies)
	}
	for messageID, count := range annotatedReplyEvents {
		if count != 1 {
			t.Fatalf("target %s must receive exactly one terminal handoff_reply update, got %d", messageID, count)
		}
	}
}

func TestRealtimeServiceAllowsReciprocalPublicMentionHandoff(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区 @ 接力测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyFirstClient := newFakeRoomClient()
	devinClient := newFakeRoomClient()
	amySecondPrompt := make(chan string, 1)
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{amyFirstClient, devinClient}}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)

	amyQueryCount := 0
	amyFirstClient.onQuery = func(_ context.Context, prompt string) error {
		amyQueryCount++
		if amyQueryCount == 1 {
			go sendFakeAssistantResult(amyFirstClient, "amy-public-mention-chain-1", "@Devin 请接下一联。")
			return nil
		}
		amySecondPrompt <- prompt
		go sendFakeAssistantResult(amyFirstClient, "amy-public-mention-chain-2", "收到，继续接力。")
		return nil
	}
	devinClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(devinClient, "devin-public-mention-chain-1", "@Amy 我接完了，你继续。")
		return nil
	}

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention-chain")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 你俩接力 5 轮",
		RoundID:        "room-round-public-mention-chain",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	select {
	case prompt := <-amySecondPrompt:
		if !strings.Contains(prompt, "<latest_trigger>\nDevin: @Amy 我接完了，你继续。") {
			t.Fatalf("reciprocal handoff 缺少 Devin 的明确触发: %s", prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("显式 reciprocal @ 没有再次唤醒 Amy")
	}

	finishedMentionRounds := 0
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeRoundStatus {
			return false
		}
		roundID, _ := event.Data["round_id"].(string)
		status, _ := event.Data["status"].(string)
		if strings.HasPrefix(roundID, "room_mention_") && status == "finished" {
			finishedMentionRounds++
		}
		return finishedMentionRounds >= 2
	})

	handoffs, err := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath).ListRoot(
		roomContext.Room.OwnerUserID,
		roomContext.Conversation.ID,
		"room-round-public-mention-chain",
	)
	if err != nil {
		t.Fatalf("读取 reciprocal handoff ledger 失败: %v", err)
	}
	if len(handoffs) != 2 {
		t.Fatalf("A → B → A 应持久化两条独立 handoff: %+v", handoffs)
	}
	for _, handoff := range handoffs {
		if handoff.Status != "finished" {
			t.Fatalf("reciprocal handoff 未随目标 runtime 收口: %+v", handoff)
		}
	}
	var amyToDevin workspacestore.RoomPublicHandoff
	for _, handoff := range handoffs {
		if handoff.SourceAgentID == amy.AgentID && handoff.TargetAgentID == devin.AgentID {
			amyToDevin = handoff
			break
		}
	}
	if amyToDevin.HandoffID == "" {
		t.Fatalf("missing Amy to Devin handoff: %+v", handoffs)
	}
	var devinReply protocol.Message
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage &&
			event.MessageID == "devin-public-mention-chain-1" &&
			event.Data["role"] == "assistant" && event.Data["is_complete"] == true &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil {
			devinReply = protocol.Message(event.Data)
			break
		}
	}
	reply := protocol.NormalizePublicHandoffReply(devinReply["handoff_reply"])
	if reply == nil || reply.HandoffID != amyToDevin.HandoffID ||
		reply.SourceMessageID != "amy-public-mention-chain-1" || reply.SourceAgentID != amy.AgentID {
		t.Fatalf("reciprocal target must retain source reply annotation: reply=%+v message=%+v", reply, devinReply)
	}
	if mentionCountFromTestValue(devinReply["agent_mentions"]) != 1 {
		t.Fatalf("explicit @Amy must coexist with handoff_reply as a new action: %+v", devinReply)
	}
}

func TestRealtimeServiceSerializesSiblingPublicMentionReturnsToFinishedHost(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	host := createTestAgent(t, agentService, ctx, "Host")
	workerA := createTestAgent(t, agentService, ctx, "WorkerA")
	workerB := createTestAgent(t, agentService, ctx, "WorkerB")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:    []string{host.AgentID, workerA.AgentID, workerB.AgentID},
		HostAgentID: host.AgentID,
		Name:        "成员并行回交 Host 测试房间",
		Title:       "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	hostClient := newFakeRoomClient()
	fastWorkerClient := newFakeRoomClient()
	slowWorkerClient := newFakeRoomClient()
	workerStarted := make(chan struct{}, 2)
	hostReturnPrompts := make(chan string, 2)
	releaseSlowWorker := make(chan struct{})
	releaseFirstHostReturn := make(chan struct{})
	var hostQueryCount atomic.Int32

	hostClient.onQuery = func(_ context.Context, prompt string) error {
		switch hostQueryCount.Add(1) {
		case 1:
			go sendFakeAssistantResult(
				hostClient,
				"host-delegates-two-workers",
				"@WorkerA 请完成 A 部分，@WorkerB 请完成 B 部分。",
			)
		case 2:
			hostReturnPrompts <- prompt
			go func() {
				<-releaseFirstHostReturn
				sendFakeAssistantResult(hostClient, "host-consumes-first-return", "第一份已收到，继续等待另一份。")
			}()
		case 3:
			hostReturnPrompts <- prompt
			go sendFakeAssistantResult(hostClient, "host-consumes-second-return", "两份回交均已收到。")
		}
		return nil
	}
	fastWorkerClient.onQuery = func(_ context.Context, _ string) error {
		workerStarted <- struct{}{}
		go sendFakeAssistantResult(fastWorkerClient, "worker-fast-return", "@Host 第一份成员回交。")
		return nil
	}
	slowWorkerClient.onQuery = func(_ context.Context, _ string) error {
		workerStarted <- struct{}{}
		go func() {
			<-releaseSlowWorker
			sendFakeAssistantResult(slowWorkerClient, "worker-slow-return", "@Host 第二份成员回交。")
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{hostClient, fastWorkerClient, slowWorkerClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := &realtimeTestSender{
		key:    "room-sender-sibling-returns-to-host",
		events: make(chan protocol.EventMessage, 512),
	}
	permission.BindSession(sharedSessionKey, sender)

	const rootRoundID = "room-round-sibling-returns-to-host"
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Host 请让两个成员并行处理后回交",
		RoundID:        rootRoundID,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	for range 2 {
		select {
		case <-workerStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("Host 委派后并非两个成员都进入 runtime")
		}
	}

	var firstReturnPrompt string
	select {
	case firstReturnPrompt = <-hostReturnPrompts:
		if !strings.Contains(firstReturnPrompt, "<latest_trigger>\n") ||
			!strings.Contains(firstReturnPrompt, "@Host 第一份成员回交。") {
			t.Fatalf("Host 首次回交 prompt 缺少第一位成员触发: %s", firstReturnPrompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host 完成初始委派后未被第一位成员再次唤醒")
	}
	if hostQueryCount.Load() != 2 {
		t.Fatalf("慢成员仍在运行时 Host 应只被再次唤醒一次: queries=%d", hostQueryCount.Load())
	}

	close(releaseSlowWorker)
	var queuedReturn protocol.InputQueueItem
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeInputQueue {
			return false
		}
		for _, item := range inputQueueItemsFromEvent(event) {
			if item.Source == protocol.InputQueueSourceAgentPublicMention &&
				item.AgentID == host.AgentID &&
				item.SourceMessageID == "worker-slow-return" {
				queuedReturn = item
				return true
			}
		}
		return false
	})
	if queuedReturn.DeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		queuedReturn.HandoffID == "" ||
		queuedReturn.RootRoundID != rootRoundID {
		t.Fatalf("第二位成员回交必须按原 root 进入 Host 串行队列: %+v", queuedReturn)
	}
	if hostQueryCount.Load() != 2 {
		t.Fatalf("Host 忙时不得并发启动第二个回交 slot: queries=%d", hostQueryCount.Load())
	}
	select {
	case prompt := <-hostReturnPrompts:
		t.Fatalf("Host 首个回交尚未结束时不应消费第二个回交: %s", prompt)
	default:
	}

	hostQueueLocation := workspacestore.InputQueueLocation{
		OwnerUserID:    roomContext.Room.OwnerUserID,
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	hostQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(hostQueueLocation)
	if err != nil {
		t.Fatalf("读取 Host 回交队列失败: %v", err)
	}
	if len(hostQueueItems) != 1 || hostQueueItems[0].ID != queuedReturn.ID {
		t.Fatalf("第二位成员回交未持久化到 Host 自己的队列: event=%+v stored=%+v", queuedReturn, hostQueueItems)
	}

	close(releaseFirstHostReturn)
	var secondReturnPrompt string
	select {
	case secondReturnPrompt = <-hostReturnPrompts:
		if !strings.Contains(secondReturnPrompt, "<latest_trigger>\n") ||
			!strings.Contains(secondReturnPrompt, "@Host 第二份成员回交。") {
			t.Fatalf("Host 第二次回交 prompt 缺少排队成员触发: %s", secondReturnPrompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host 首个回交结束后未串行消费第二位成员回交")
	}
	if hostQueryCount.Load() != 3 {
		t.Fatalf("Host 应按初始委派、第一份回交、第二份回交串行执行三次: queries=%d", hostQueryCount.Load())
	}

	handoffStore := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath)
	var handoffs []workspacestore.RoomPublicHandoff
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		handoffs, err = handoffStore.ListRoot(
			roomContext.Room.OwnerUserID,
			roomContext.Conversation.ID,
			rootRoundID,
		)
		if err != nil {
			t.Fatalf("读取成员回交 handoff ledger 失败: %v", err)
		}
		allFinished := len(handoffs) == 4
		for _, handoff := range handoffs {
			allFinished = allFinished && handoff.Status == "finished"
		}
		if allFinished {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(handoffs) != 4 {
		t.Fatalf("Host 委派两次且成员回交两次应产生四条 handoff: %+v", handoffs)
	}
	returnRoundIDs := make(map[string]struct{}, 2)
	returnSources := make(map[string]struct{}, 2)
	for _, handoff := range handoffs {
		if handoff.Status != "finished" {
			t.Fatalf("成员协作 handoff 未收口: %+v", handoff)
		}
		if handoff.TargetAgentID != host.AgentID {
			continue
		}
		if handoff.TargetRoundID == "" {
			t.Fatalf("回交 Host 的 handoff 没有实际 target round: %+v", handoff)
		}
		returnRoundIDs[handoff.TargetRoundID] = struct{}{}
		returnSources[handoff.SourceAgentID] = struct{}{}
	}
	if len(returnRoundIDs) != 2 || len(returnSources) != 2 {
		t.Fatalf("两个来源回交同一 Host 必须各自串行启动独立 round: %+v", handoffs)
	}
}

func TestRealtimeServiceQueuesPublicMentionWhenTargetRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区 @ 排队房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	amyClient := newFakeRoomClient()
	devinQueuedPrompt := make(chan string, 1)
	devinQueryCount := 0
	devinCurrentClient.onQuery = func(_ context.Context, prompt string) error {
		devinQueryCount++
		if devinQueryCount == 1 {
			return nil
		}
		devinQueuedPrompt <- prompt
		go sendFakeAssistantResult(devinCurrentClient, "devin-public-mention-after-busy", "天气任务已处理。")
		return nil
	}
	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(amyClient, "amy-public-mention-busy", "@Devin 当前天气任务交给你。")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient, amyClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention-queue")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	devinActiveRoundID := ""
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType == protocol.EventTypeStreamStart && event.AgentID == devin.AgentID {
			devinActiveRoundID = event.AgentRoundID
			return true
		}
		return false
	})

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 查下天气",
		RoundID:        "room-round-amy-mentions-busy-devin",
	}); err != nil {
		t.Fatalf("启动 Amy 公区 @ 失败: %v", err)
	}

	var queuedItem protocol.InputQueueItem
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeInputQueue {
			return false
		}
		for _, item := range inputQueueItemsFromEvent(event) {
			if item.Source == protocol.InputQueueSourceAgentPublicMention && item.AgentID == devin.AgentID {
				queuedItem = item
				return true
			}
		}
		return false
	})
	if queuedItem.SourceMessageID != "amy-public-mention-busy" ||
		queuedItem.SourceAgentID != amy.AgentID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID ||
		queuedItem.DeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		queuedItem.RootRoundID == devinActiveRoundID {
		t.Fatalf("公区 @ 队列项缺少来源或目标: %+v", queuedItem)
	}
	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 || targetQueueItems[0].ID != queuedItem.ID ||
		targetQueueItems[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		targetQueueItems[0].RootRoundID == devinActiveRoundID {
		t.Fatalf("Room 队列未落到目标 agent session: event=%+v stored=%+v", queuedItem, targetQueueItems)
	}

	devinCurrentClient.mu.Lock()
	interruptCalls := devinCurrentClient.interruptCalls
	devinCurrentClient.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("公区 @ 不应中断正在工作的目标 agent: interruptCalls=%d", interruptCalls)
	}
	select {
	case prompt := <-devinQueuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued mention: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-task-done", "当前长任务完成。")
	select {
	case prompt := <-devinQueuedPrompt:
		if !strings.Contains(prompt, "<latest_trigger>\nAmy: @Devin 当前天气任务交给你。") {
			t.Fatalf("queued mention prompt 缺少公区 @ 触发上下文: %s", prompt)
		}
		if strings.Contains(prompt, "type:") || strings.Contains(prompt, "fanout_targets:") {
			t.Fatalf("queued mention prompt 不应包含字段化 trigger: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued mention")
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeRoundStatus {
			return false
		}
		roundID, _ := event.Data["round_id"].(string)
		status, _ := event.Data["status"].(string)
		return strings.HasPrefix(roundID, "room_mention_") && status == "finished"
	})
}
