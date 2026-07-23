package realtime_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
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

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

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

func TestRoomServiceTouchConversationActivityOrdersContexts(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

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
	if _, err = roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{
		Title: "后创建的话题",
	}); err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}

	activityAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	if err = roomService.TouchConversationActivity(ctx, mainContext.Conversation.ID, activityAt); err != nil {
		t.Fatalf("更新 conversation 活动时间失败: %v", err)
	}

	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取 room contexts 失败: %v", err)
	}
	if len(contexts) < 2 {
		t.Fatalf("room contexts 数量不正确: got=%d want>=2", len(contexts))
	}
	if contexts[0].Conversation.ID != mainContext.Conversation.ID {
		t.Fatalf("最近活动 conversation 未排第一: got=%s want=%s", contexts[0].Conversation.ID, mainContext.Conversation.ID)
	}
	if contexts[0].Conversation.LastActivityAt.Before(activityAt.Add(-time.Second)) {
		t.Fatalf("conversation last_activity_at 未写入: got=%s want>=%s", contexts[0].Conversation.LastActivityAt, activityAt)
	}

	if err = roomService.TouchConversationActivity(ctx, mainContext.Conversation.ID, activityAt.Add(-time.Hour)); err != nil {
		t.Fatalf("重复更新 conversation 活动时间失败: %v", err)
	}
	contexts, err = roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("重新读取 room contexts 失败: %v", err)
	}
	if contexts[0].Conversation.LastActivityAt.Before(activityAt.Add(-time.Second)) {
		t.Fatalf("conversation last_activity_at 被旧时间回退: got=%s want>=%s", contexts[0].Conversation.LastActivityAt, activityAt)
	}
}

func TestRoomServiceClosesConversationRuntime(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	runtimeCloser := &fakeRoomRuntimeCloser{}
	roomService.SetRuntimeManager(runtimeCloser)

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

func TestRealtimeServiceHandleInterruptCancelsAllSlots(t *testing.T) {
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
	agentA := createTestAgent(t, agentService, ctx, "助手甲")
	agentB := createTestAgent(t, agentService, ctx, "助手乙")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "中断测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	clientA := newFakeRoomClient()
	clientA.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientA.onInterrupt = func(_ context.Context) {
		go func() {
			clientA.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientA.sessionID,
				UUID:      "room-interrupted-a",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientB.onInterrupt = func(_ context.Context) {
		go func() {
			clientB.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientB.sessionID,
				UUID:      "room-interrupted-b",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{clientA, clientB}},
	)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-2")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 @助手乙 处理一下",
		RoundID:        "room-round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return countEventType(events, protocol.EventTypeStreamStart) >= 2
	})

	if err = service.HandleInterrupt(ctx, realtimesvc.InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	if countEventType(events, protocol.EventTypeAgentRoundStatus) < 2 {
		t.Fatalf("期望每个 slot 都产出 interrupted 状态: %+v", events)
	}
	if countRoomResultSubtype(events, "interrupted") != 0 {
		t.Fatalf("空 interrupted result 不应形成公开气泡: %+v", events)
	}

	clientA.mu.Lock()
	interruptA := clientA.interruptCalls
	clientA.mu.Unlock()
	clientB.mu.Lock()
	interruptB := clientB.interruptCalls
	clientB.mu.Unlock()
	if interruptA == 0 || interruptB == 0 {
		t.Fatalf("所有 slot 都应收到 interrupt: a=%d b=%d", interruptA, interruptB)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	sharedInterrupted := 0
	for _, message := range sharedMessages {
		summary, ok := message["result_summary"].(map[string]any)
		if ok && summary["subtype"] == "interrupted" {
			sharedInterrupted++
		}
	}
	if sharedInterrupted < 2 {
		t.Fatalf("共享日志未完整落 interrupted result: %+v", sharedMessages)
	}

	for _, agentValue := range []*protocol.Agent{agentA, agentB} {
		privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType)
		writeRoomTranscriptFixture(t, agentValue.WorkspacePath, "room-sdk-session", []map[string]any{
			{
				"type":      "user",
				"uuid":      "interrupt-user-" + agentValue.AgentID,
				"sessionId": "room-sdk-session",
				"timestamp": "2026-04-19T18:20:00Z",
				"message": map[string]any{
					"role":    "user",
					"content": "dispatch prompt",
				},
			},
		})
		privateMessages := readRoomPrivateHistory(
			t,
			cfg.WorkspacePath,
			agentValue.WorkspacePath,
			privateSessionKey,
			agentValue.AgentID,
			"room-sdk-session",
		)
		foundInterrupted := false
		for _, message := range privateMessages {
			summary, ok := message["result_summary"].(map[string]any)
			if ok && summary["subtype"] == "interrupted" {
				foundInterrupted = true
				break
			}
		}
		if !foundInterrupted {
			t.Fatalf("私有日志未落 interrupted result: agent=%s messages=%+v", agentValue.AgentID, privateMessages)
		}
	}
}

func TestRealtimeServiceTreatsClosedStreamAfterInterruptAsInterrupted(t *testing.T) {
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
		return event.EventType == protocol.EventTypeStreamStart
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

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	foundInterrupted := false
	for _, message := range sharedMessages {
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
}
