package realtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

type fakeRoomQuotaChecker struct {
	err error
}

type testRoomGoalSessionOwnershipVerifier struct {
	agentNames map[string]string
}

func (v testRoomGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request goalsvc.GoalSessionOwnershipRequest,
) (goalsvc.GoalSessionOwnershipProof, error) {
	agentID := request.TrustedAgentID
	return goalsvc.GoalSessionOwnershipProof{
		TrustedAgentID:   agentID,
		TrustedAgentName: v.agentNames[agentID],
	}, nil
}

func TestRealtimeServiceHandleChatMarksInternalGoalUsageLimitedWhenQuotaExceeded(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-internal-goal-quota",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "内部 Goal 额度助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	factory := &fakeRoomFactory{clients: []*fakeRoomClient{newFakeRoomClient()}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
	)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{memberAgent.AgentID: memberAgent.Name},
	})
	service.SetGoalContextProvider(goalService)
	quotaErr := subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}
	service.SetQuotaChecker(fakeRoomQuotaChecker{err: quotaErr})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	goal, err := goalService.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  sharedSessionKey,
		Objective:   "保持账号额度边界",
		OwnerUserID: "owner-room-internal-goal-quota",
	})
	if err != nil {
		t.Fatalf("创建 Room Goal 失败: %v", err)
	}
	err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "internal goal continuation",
		GoalID:         goal.ID,
		RoundID:        "room-round-internal-goal-quota",
		Internal:       true,
	})
	if !errors.Is(err, subscriptionsvc.ErrQuotaExceeded) {
		t.Fatalf("账号额度耗尽应阻止内部 Room Goal runtime，实际: %v", err)
	}
	limited, err := goalService.Current(ctx, sharedSessionKey)
	if err != nil {
		t.Fatalf("读取受限 Room Goal 失败: %v", err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.LastError != quotaErr.ClientMessage() {
		t.Fatalf("受限 Room Goal = %#v, want usage_limited with actionable subscription quota message", limited)
	}
	if got := factory.LastOptions(); got.Model != "" {
		t.Fatalf("账号额度耗尽时不应创建内部 Room Goal runtime: %+v", got)
	}
}

func (f fakeRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestRealtimeServiceHandleChatBlocksRuntimeWhenQuotaExceeded(t *testing.T) {
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
		UserID:   "owner-room-quota",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "额度助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	factory := &fakeRoomFactory{clients: []*fakeRoomClient{newFakeRoomClient()}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
	)
	errQuota := errors.New("quota exceeded")
	service.SetQuotaChecker(fakeRoomQuotaChecker{err: errQuota})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "你好",
		RoundID:        "room-round-quota",
	})
	if !errors.Is(err, errQuota) {
		t.Fatalf("quota exceeded 应阻止 Room runtime，实际: %v", err)
	}
	if got := factory.LastOptions(); got.Model != "" {
		t.Fatalf("quota exceeded 时不应创建 Room runtime: %+v", got)
	}
}

func TestRealtimeServiceCompletesRoomRoundFromTerminalAssistantWithoutResult(t *testing.T) {
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
		UserID:   "user-room-assistant-terminal",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "终态助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "assistant 终态房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeTerminalAssistantAndClose(client, "assistant-terminal-no-result", "这是完整回复。", map[string]any{
			"input_tokens":  7,
			"output_tokens": 11,
		})
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
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
	sender := newRealtimeTestSender("room-sender-assistant-terminal")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@终态助手 写一句话",
		RoundID:        "room-round-assistant-terminal",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeError || event.Data["status"] == "error" {
			t.Fatalf("assistant end_turn 无 result 不应进入错误态: %+v", event)
		}
	}
	assistantPayload := findRoomAssistantMessagePayload(t, events, "assistant-terminal-no-result")
	if assistantPayload["is_complete"] != true || assistantPayload["stop_reason"] != "end_turn" {
		t.Fatalf("terminal assistant 事件应保持完整终态: %+v", assistantPayload)
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-assistant-terminal")
	if err != nil {
		t.Fatalf("读取 room token usage 失败: %v", err)
	}
	if usageSummary.InputTokens != 7 || usageSummary.OutputTokens != 11 || usageSummary.TotalTokens != 18 {
		t.Fatalf("assistant fallback usage 未写入 ledger: %+v", usageSummary)
	}
}

func TestRealtimeServiceKeepsSubagentRoomSlotInRuntimeManager(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "子任务助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "subagent runtime 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeTaskStarted,
				SessionID: client.sessionID,
				TaskStarted: &sdkprotocol.TaskStartedMessage{
					TaskID:      "task-room-1",
					AgentID:     "sdk-subagent-1",
					AgentType:   "worker",
					Description: "检查 Room runtime",
					TaskType:    "local_agent",
					Additional: map[string]any{
						"child_session_id": "child-room-1",
						"name":             "Room runtime audit",
					},
				},
			}
			sendFakeAssistantResult(client, "assistant-room-subagent", "子任务已在后台执行。")
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-subagent-runtime")
	permission.BindSession(sharedSessionKey, sender)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@子任务助手 启动后台检查",
		RoundID:        "room-round-subagent-runtime",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		memberAgent.AgentID,
		roomContext.Room.RoomType,
	)
	if !runtimeManager.HasSubagentHistory(runtimeSessionKey) {
		t.Fatal("Room slot 未在 runtime manager 保留 subagent history")
	}
	if rounds := runtimeManager.GetRunningRoundIDs(runtimeSessionKey); len(rounds) != 0 {
		t.Fatalf("父 round 结束后仍残留 running round: %+v", rounds)
	}
	if err = runtimeManager.StopTask(ctx, runtimeSessionKey, "task-room-1"); err != nil {
		t.Fatalf("Room task stop 未路由到 slot client: %v", err)
	}
	if err = runtimeManager.SendTaskMessage(ctx, runtimeSessionKey, "task-room-1", "继续检查", "继续检查"); err != nil {
		t.Fatalf("Room task follow-up 未路由到 slot client: %v", err)
	}
	client.mu.Lock()
	if client.disconnects != 0 || len(client.stoppedTasks) != 1 || len(client.taskMessages) != 1 {
		t.Fatalf("Room slot client 生命周期/控制不正确: disconnects=%d stopped=%+v messages=%+v", client.disconnects, client.stoppedTasks, client.taskMessages)
	}
	client.mu.Unlock()

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room history 失败: %v", err)
	}
	var taskMetadata map[string]any
	for _, message := range messages {
		metadata, _ := message["metadata"].(map[string]any)
		if metadata["task_id"] == "task-room-1" {
			taskMetadata = metadata
			break
		}
	}
	if taskMetadata["runtime_kind"] != "nxs" || taskMetadata["child_session_id"] != "child-room-1" {
		t.Fatalf("Room task metadata 未保留 runtime/thread 身份: %+v", taskMetadata)
	}
	if err = runtimeManager.CloseSession(ctx, runtimeSessionKey); err != nil {
		t.Fatalf("清理 Room runtime 失败: %v", err)
	}
}

func TestRealtimeServiceKeepsThinkingDuringStreamingAndHistoryReplay(t *testing.T) {
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
	memberAgent := createTestAgent(t, agentService, ctx, "流式思考助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-room-think-1",
							"model": "sonnet",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type":     "thinking",
							"thinking": "先分析",
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
							"type":     "thinking_delta",
							"thinking": " 再收口",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "今天天气",
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
							"text": " 很不错",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-room-think-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "今天天气 很不错"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-room-think-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
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
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-think-stream")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "今天天气怎么样呀",
		RoundID:        "room-round-think-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertRoomStreamBlockIndex(t, events, "assistant-room-think-1", "thinking", 0)
	assertRoomStreamBlockIndex(t, events, "assistant-room-think-1", "text", 1)

	assistantPayload := findRoomAssistantMessagePayload(t, events, "assistant-room-think-1")
	assistantBlocks := roomContentBlocksFromPayload(t, assistantPayload)
	if len(assistantBlocks) != 2 {
		t.Fatalf("Room durable assistant 内容块数量不正确: %+v", assistantPayload)
	}
	if assistantBlocks[0]["type"] != "thinking" || assistantBlocks[0]["thinking"] != "先分析 再收口" {
		t.Fatalf("Room durable assistant 未保留完整 thinking: %+v", assistantBlocks)
	}
	if assistantBlocks[1]["type"] != "text" || assistantBlocks[1]["text"] != "今天天气 很不错" {
		t.Fatalf("Room durable assistant 未保留 text: %+v", assistantBlocks)
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, memberAgent.AgentID, roomContext.Room.RoomType)
	roomThinkingTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-think-user-1",
			"sessionId": client.sessionID,
			"timestamp": roomThinkingTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-room-think-1",
			"sessionId":  client.sessionID,
			"parentUuid": "room-think-user-1",
			"timestamp":  roomThinkingTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先分析 再收口"},
					{"type": "text", "text": "今天天气 很不错"},
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
	sharedBlocks := roomContentBlocksFromPayload(t, sharedMessages[1])
	if len(sharedBlocks) != 2 || sharedBlocks[0]["type"] != "thinking" || sharedBlocks[1]["type"] != "text" {
		t.Fatalf("共享历史 assistant 内容块不正确: %+v", sharedMessages[1])
	}
	if _, exists := sharedMessages[1]["stream_status"]; exists {
		t.Fatalf("共享历史 assistant 不应携带 stream_status: %+v", sharedMessages[1])
	}
	if _, ok := sharedMessages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("共享历史 assistant 应挂载 result 摘要: %+v", sharedMessages[1])
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
	privateBlocks := roomContentBlocksFromPayload(t, privateMessages[1])
	if len(privateBlocks) != 2 || privateBlocks[0]["type"] != "thinking" || privateBlocks[1]["type"] != "text" {
		t.Fatalf("私有历史 assistant 内容块不正确: %+v", privateMessages[1])
	}
	if _, exists := privateMessages[1]["stream_status"]; exists {
		t.Fatalf("私有历史 assistant 不应携带 stream_status: %+v", privateMessages[1])
	}
	if _, ok := privateMessages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("私有历史 assistant 应挂载 result 摘要: %+v", privateMessages[1])
	}
}
