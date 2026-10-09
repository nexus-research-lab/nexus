package realtime_test

import (
	"context"
	"errors"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
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
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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

func TestRealtimeServiceCompletesRoomRoundFromTerminalAssistantWithoutResult(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
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
