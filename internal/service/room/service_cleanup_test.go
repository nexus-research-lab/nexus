package room_test

import (
	"context"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestRoomServiceCleansRoomArtifacts(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	goalCleaner := &fakeRoomGoalCleaner{}
	roomService.SetGoalCleaner(goalCleaner)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "清理助手A")
	agentB := createTestAgent(t, agentService, ctx, "清理助手B")
	agentC := createTestAgent(t, agentService, ctx, "清理助手C")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "清理测试 room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{
		Title: "待删除话题",
	})
	if err != nil {
		t.Fatalf("创建话题失败: %v", err)
	}
	if _, err = roomService.AddRoomMember(ctx, mainContext.Room.ID, protocol.AddRoomMemberRequest{AgentID: agentC.AgentID}); err != nil {
		t.Fatalf("追加成员失败: %v", err)
	}

	contextsAfterAdd, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取房间上下文失败: %v", err)
	}
	mainContextAfterAdd, ok := findConversationContext(contextsAfterAdd, mainContext.Conversation.ID)
	if !ok {
		t.Fatalf("未找到主对话上下文")
	}
	topicContextAfterAdd, ok := findConversationContext(contextsAfterAdd, topicContext.Conversation.ID)
	if !ok {
		t.Fatalf("未找到 topic 上下文")
	}

	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	paths := workspacestore.New(cfg.WorkspacePath)

	mainAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentA.AgentID)
	mainAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentB.AgentID)
	topicAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentA.AgentID)
	topicAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentB.AgentID)
	mainAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentC.AgentID)
	topicAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentC.AgentID)
	seedRoomConversationLog(t, cfg.WorkspacePath, mainContextAfterAdd.Conversation.ID, mainContextAfterAdd.Room.ID)
	seedRoomConversationLog(t, cfg.WorkspacePath, topicContextAfterAdd.Conversation.ID, topicContextAfterAdd.Room.ID)
	mainAgentCDBSessionID := findRoomSessionID(t, mainContextAfterAdd, agentC.AgentID)
	_, mainAgentCRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		mainContextAfterAdd.Conversation.ID,
		mainAgentCDBSessionID,
		"remove-member",
	)
	mainAgentADBSessionID := findRoomSessionID(t, mainContextAfterAdd, agentA.AgentID)
	_, mainRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		mainContextAfterAdd.Conversation.ID,
		mainAgentADBSessionID,
		"delete-room",
	)
	topicAgentADBSessionID := findRoomSessionID(t, topicContextAfterAdd, agentA.AgentID)
	_, topicRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		topicContextAfterAdd.Conversation.ID,
		topicAgentADBSessionID,
		"delete-topic",
	)

	if _, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID); err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, mainAgentCSession))
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, topicAgentCSession))
	assertPathExists(t, paths.RoomConversationDir(topicContextAfterAdd.Conversation.ID))
	assertSQLCount(t, db, `
SELECT COUNT(*) FROM sessions
WHERE conversation_id = ? AND agent_id = ?`, 0, mainContextAfterAdd.Conversation.ID, agentC.AgentID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, mainAgentCRoundID)
	assertRoomGoalMemberCleanup(t, goalCleaner, 0, agentC.AgentID, []string{
		mainContextAfterAdd.Conversation.ID,
		topicContextAfterAdd.Conversation.ID,
	})

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContextAfterAdd.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ID != mainContextAfterAdd.Conversation.ID {
		t.Fatalf("删除 topic 后未回退到主对话: %+v", fallbackContext.Conversation)
	}
	assertPathRemoved(t, paths.RoomConversationDir(topicContextAfterAdd.Conversation.ID))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, topicAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, topicAgentBSession))
	assertPathExists(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathExists(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, topicRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 0, []string{topicContextAfterAdd.Conversation.ID})

	if err = roomService.DeleteRoom(ctx, mainContext.Room.ID); err != nil {
		t.Fatalf("删除 room 失败: %v", err)
	}
	assertPathRemoved(t, paths.RoomConversationDir(mainContextAfterAdd.Conversation.ID))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rooms WHERE id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM members WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, mainRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 1, []string{mainContextAfterAdd.Conversation.ID})
}
