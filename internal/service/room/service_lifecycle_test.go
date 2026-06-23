package room_test

import (
	"context"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"

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
