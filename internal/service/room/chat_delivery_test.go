package room_test

import (
	"context"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServiceHandleChatWithDirectRoomFallbackTarget(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
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
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
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
	service := roomsvc.NewRealtimeServiceWithFactory(
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

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-1")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "你好",
		RoundID:        "room-round-1",
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
		"<nexus_runtime_context>",
		"## Emotion State",
		"Base: focused",
	} {
		if !strings.Contains(queryPrompts[0], expected) {
			t.Fatalf("Room runtime query 缺少动态上下文 %q:\n%s", expected, queryPrompts[0])
		}
	}

	pendingMsgID := ""
	for _, event := range events {
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

	roomSystemPrompt := factory.LastOptions().System.Append
	for _, expected := range []string{
		"# Nexus Room",
		"You are a member in a multi-member Nexus Room",
		"Each user turn includes <public_feed>",
		"Private Room directed message sending is disabled",
		`nexus_room.publish_public_message`,
		`"room host default takeover"`,
		"When a directed message wakes you, answer once in the final reply",
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
		`nexus_room.send_directed_message`,
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
	if got := strings.TrimSpace(factory.LastOptions().Env["NEXUSCTL_COMMAND_PATH"]); got == "" {
		t.Fatalf("Room runtime 应注入明确 nexusctl 命令路径: %+v", factory.LastOptions().Env)
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(dmContext.Conversation.ID, memberAgent.AgentID, dmContext.Room.RoomType)
	cursor, ok, err := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ReadRoomPublicCursor(
		memberAgent.WorkspacePath,
		privateSessionKey,
		dmContext.Conversation.ID,
		memberAgent.AgentID,
	)
	if err != nil {
		t.Fatalf("读取 Room 公区 cursor 失败: %v", err)
	}
	if !ok || !strings.HasPrefix(cursor.LastPublicMessageID, "msg_user_") {
		t.Fatalf("成功 round 应记录目标 agent 公区消费位置: ok=%v cursor=%+v", ok, cursor)
	}
	roomTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
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
	sharedMessages, err := roomHistory.ReadMessages(dmContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 2 {
		t.Fatalf("共享消息数量不正确: got=%d want=2", len(sharedMessages))
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

	agentService, db, err := serverapp.NewAgentService(cfg)
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
	service := roomsvc.NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimectx.NewManager(), permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-host-default")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
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
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
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

	agentService, db, err := serverapp.NewAgentService(cfg)
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
	service := NewRealtimeServiceWithFactory(
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

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
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

	agentService, db, err := serverapp.NewAgentService(cfg)
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
			sendFakeAssistantResultWithUsage(client, "amy-no-reply", "<nexus_room_no_reply/>", map[string]any{
				"input_tokens":  7,
				"output_tokens": 2,
			})
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
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

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
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
	usageSummary, err := usageService.Summary(ctx, "user-room-no-reply")
	if err != nil {
		t.Fatalf("读取 no-reply token usage 失败: %v", err)
	}
	if usageSummary.TotalTokens != 9 {
		t.Fatalf("no-reply result usage 也应写入 ledger: %+v", usageSummary)
	}
}
