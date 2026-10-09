package realtime_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoomServiceLifecycle(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	roomService.SetSessionArtifactDeletionCoordinator(
		noopSessionArtifactDeletionCoordinator{},
	)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	agentC := createTestAgent(t, agentService, ctx, "测试助手C")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{agentA.AgentID, agentB.AgentID},
		Name:                   "产品讨论",
		Title:                  "主对话",
		Avatar:                 "7",
		HostAgentID:            agentA.AgentID,
		HostAutoReplyEnabled:   true,
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if mainContext.Room.RoomType != protocol.RoomTypeGroup {
		t.Fatalf("room_type 不正确: %s", mainContext.Room.RoomType)
	}
	if mainContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("主对话类型不正确: %s", mainContext.Conversation.ConversationType)
	}
	if len(mainContext.Members) != 3 {
		t.Fatalf("成员数量不正确: got=%d want=3", len(mainContext.Members))
	}
	if len(mainContext.Sessions) != 2 {
		t.Fatalf("主对话 session 数量不正确: got=%d want=2", len(mainContext.Sessions))
	}
	if mainContext.Room.Avatar != "7" {
		t.Fatalf("room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, "7")
	}
	if mainContext.Room.HostAgentID != agentA.AgentID || !mainContext.Room.HostAutoReplyEnabled {
		t.Fatalf("room 群主设置不正确: %+v", mainContext.Room)
	}
	if !mainContext.Room.PrivateMessagesEnabled {
		t.Fatalf("room 私信设置不正确: %+v", mainContext.Room)
	}

	rooms, err := roomService.ListRooms(ctx, 20)
	if err != nil {
		t.Fatalf("列出 room 失败: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("room 数量不正确: got=%d want=1", len(rooms))
	}
	if rooms[0].Room.Avatar != "7" {
		t.Fatalf("list room avatar 不正确: got=%q want=%q", rooms[0].Room.Avatar, "7")
	}
	if !rooms[0].Room.PrivateMessagesEnabled {
		t.Fatalf("list room 私信设置不正确: %+v", rooms[0].Room)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}

	updatedAvatar := "12"
	disableHostAutoReply := false
	disablePrivateMessages := false
	nextHostAgentID := agentB.AgentID
	mainContext, err = roomService.UpdateRoom(ctx, mainContext.Room.ID, protocol.UpdateRoomRequest{
		Avatar:                 &updatedAvatar,
		HostAgentID:            &nextHostAgentID,
		HostAutoReplyEnabled:   &disableHostAutoReply,
		PrivateMessagesEnabled: &disablePrivateMessages,
	})
	if err != nil {
		t.Fatalf("更新 room avatar 失败: %v", err)
	}
	if mainContext.Room.Avatar != updatedAvatar {
		t.Fatalf("更新后 room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, updatedAvatar)
	}
	if mainContext.Room.HostAgentID != agentB.AgentID || mainContext.Room.HostAutoReplyEnabled {
		t.Fatalf("更新后 room 群主设置不正确: %+v", mainContext.Room)
	}
	if mainContext.Room.PrivateMessagesEnabled {
		t.Fatalf("更新后 room 私信设置不正确: %+v", mainContext.Room)
	}

	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{})
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	if topicContext.Conversation.ConversationType != protocol.ConversationTypeTopic {
		t.Fatalf("topic 类型不正确: %s", topicContext.Conversation.ConversationType)
	}
	if len(topicContext.Sessions) != 2 {
		t.Fatalf("topic session 数量不正确: got=%d want=2", len(topicContext.Sessions))
	}

	updatedContext, err := roomService.AddRoomMember(ctx, mainContext.Room.ID, protocol.AddRoomMemberRequest{
		AgentID: agentC.AgentID,
	})
	if err != nil {
		t.Fatalf("追加成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 3 {
		t.Fatalf("追加成员后主对话 session 数量不正确: got=%d want=3", len(updatedContext.Sessions))
	}

	updatedContext, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID)
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 2 {
		t.Fatalf("移除成员后主对话 session 数量不正确: got=%d want=2", len(updatedContext.Sessions))
	}

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContext.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("删除 topic 后未回退到主对话: %s", fallbackContext.Conversation.ConversationType)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊失败: %v", err)
	}
	if dmContext.Room.RoomType != protocol.RoomTypeDM {
		t.Fatalf("直聊类型不正确: %s", dmContext.Room.RoomType)
	}
	if len(dmContext.Sessions) != 1 {
		t.Fatalf("直聊 session 数量不正确: got=%d want=1", len(dmContext.Sessions))
	}
}

func TestRoomServiceClosesConversationRuntime(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	runtimeCloser := &fakeRoomRuntimeCloser{}
	roomService.SetRuntimeManager(runtimeCloser)
	roomService.SetSessionArtifactDeletionCoordinator(
		noopSessionArtifactDeletionCoordinator{},
	)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "产品讨论",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{})
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	expectedKeys := []string{
		protocol.BuildRoomSharedSessionKey(topicContext.Conversation.ID),
		protocol.BuildRoomAgentSessionKey(topicContext.Conversation.ID, agentA.AgentID, protocol.RoomTypeGroup),
		protocol.BuildRoomAgentSessionKey(topicContext.Conversation.ID, agentB.AgentID, protocol.RoomTypeGroup),
	}

	if err = roomService.CloseConversationRuntime(ctx, mainContext.Room.ID, topicContext.Conversation.ID); err != nil {
		t.Fatalf("关闭 conversation runtime 失败: %v", err)
	}
	assertRuntimeClosedKeys(t, runtimeCloser.keys, expectedKeys)

	runtimeCloser.keys = nil
	if _, err = roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContext.Conversation.ID); err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	assertRuntimeClosedKeys(t, runtimeCloser.keys, expectedKeys)
}

// 中断生命周期测试。

func TestRealtimeServiceTreatsClosedStreamAfterInterruptAsInterrupted(t *testing.T) {
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

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "中断关流测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeRoomThinkingStream(client, "assistant-interrupted-closed-stream", "关流前的内部思考")
		return nil
	}
	var closeOnce sync.Once
	client.onInterrupt = func(_ context.Context) {
		closeOnce.Do(func() {
			close(client.messages)
		})
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-interrupt-closed-stream")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 处理一下",
		RoundID:        "room-round-closed-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		for _, observed := range events {
			if observed.EventType != protocol.EventTypeStream {
				continue
			}
			block, _ := observed.Data["content_block"].(map[string]any)
			if block["type"] == "thinking" {
				return true
			}
		}
		return false
	})

	if err = service.HandleInterrupt(ctx, realtimesvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			(event.Data["status"] == "interrupted" || event.Data["status"] == "error")
	})
	terminalStatus := ""
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus {
			terminalStatus = anyToString(event.Data["status"])
		}
	}
	if terminalStatus != "interrupted" {
		t.Fatalf("主动中断后的关流应归类为 interrupted，实际 status=%s events=%+v", terminalStatus, events)
	}
	if countRoomResultSubtype(events, "error") > 0 {
		t.Fatalf("主动中断后的关流不应广播 error result: %+v", events)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	foundInterrupted := false
	partialThinkingPreserved := false
	for _, message := range sharedMessages {
		if message["role"] == "assistant" {
			for _, block := range roomContentBlocksFromPayload(t, message) {
				if block["type"] == "thinking" && block["thinking"] == "关流前的内部思考" {
					partialThinkingPreserved = true
				}
			}
		}
		summary, ok := message["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "error" {
			t.Fatalf("主动中断后的共享日志不应落 error summary: %+v", sharedMessages)
		}
		if summary["subtype"] == "interrupted" {
			foundInterrupted = true
			if strings.Contains(anyToString(summary["result"]), "round stream closed before terminal") {
				t.Fatalf("interrupted summary 不应暴露底层 stream 错误: %+v", summary)
			}
		}
	}
	if !foundInterrupted {
		t.Fatalf("共享日志未落 interrupted summary: %+v", sharedMessages)
	}
	if !partialThinkingPreserved {
		t.Fatalf("共享日志未保留强制中断前的流式思考: %+v", sharedMessages)
	}
}

func TestRoomRoundCompletionReachesAutomationObserver(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	agent := createTestAgent(t, agentService, ctx, "定时助手")
	room, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agent.AgentID}, Name: "自动化终态回归",
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeRoomClient()
	client.onQuery = func(context.Context, string) error {
		client.messages <- sdkprotocol.ReceivedMessage{
			Type: sdkprotocol.MessageTypeResult, SessionID: client.sessionID, UUID: "result-1",
			Result: &sdkprotocol.ResultMessage{Subtype: "success", Result: "任务完成", NumTurns: 1},
		}
		return nil
	}
	manager := runtimectx.NewManager()
	service := NewServiceWithFactory(cfg, roomService, agentService, manager,
		permissionctx.NewContext(), &fakeRoomFactory{clients: []*fakeRoomClient{client}})
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)
	sink := automationexec.NewExecutionSink("automation:completion-test")
	defer sink.Close()
	const roundID = "automation-room-round"
	sessionKey := protocol.BuildRoomSharedSessionKey(room.Conversation.ID)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey: sessionKey, ConversationID: room.Conversation.ID, RoomID: room.Room.ID,
		Content: "完成本次任务", TargetAgentIDs: []string{agent.AgentID}, RoundID: roundID,
		ExecutionOrigin: "automation",
		EventObserver:   func(ctx context.Context, event protocol.EventMessage) { _ = sink.SendEvent(ctx, event) },
	}); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)
	if observation.Status != "succeeded" {
		t.Fatalf("Room 已结束时自动化必须收到成功终态，实际 %+v", observation)
	}
	terminalCount := 0
	for _, event := range broadcaster.Events() {
		if event.EventType == protocol.EventTypeRoundStatus && event.Data["is_terminal"] == true {
			terminalCount++
		}
	}
	if terminalCount != 1 {
		t.Fatalf("Web 应收到一次 root 终态，实际 %d", terminalCount)
	}
}
