package room_test

import (
	"context"
	"errors"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestRoomServiceCleansRoomArtifacts(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
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
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
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
	coordinator := &fakeRoomSessionArtifactDeletionCoordinator{
		deleteFn: func(_ context.Context, call roomSessionArtifactDeletionCall) error {
			_, deleteErr := files.ForOwner(call.ownerUserID).DeleteSession(
				call.workspacePath,
				call.sessionKey,
			)
			return deleteErr
		},
	}
	roomService.SetSessionArtifactDeletionCoordinator(coordinator)

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
	if err = roomService.UpdateSessionRuntimeIdentity(
		ctx,
		topicAgentADBSessionID,
		"sdk-topic-agent-a",
		"",
	); err != nil {
		t.Fatalf("写入待清理 SDK session_id 失败: %v", err)
	}
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
	assertPathExists(t, paths.RoomConversationDir(
		topicContextAfterAdd.Room.OwnerUserID,
		topicContextAfterAdd.Conversation.ID,
	))
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
	assertPathRemoved(t, paths.RoomConversationDir(
		topicContextAfterAdd.Room.OwnerUserID,
		topicContextAfterAdd.Conversation.ID,
	))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, topicAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, topicAgentBSession))
	assertPathExists(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathExists(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, topicRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 0, []string{topicContextAfterAdd.Conversation.ID})

	currentRoom, err := roomService.GetRoom(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取待删除 Room 当前版本失败: %v", err)
	}
	if err = roomService.DeleteRoomAtVersion(
		ctx,
		mainContext.Room.ID,
		currentRoom.Room.ConfigurationVersion,
	); err != nil {
		t.Fatalf("删除 room 失败: %v", err)
	}
	assertPathRemoved(t, paths.RoomConversationDir(
		mainContextAfterAdd.Room.OwnerUserID,
		mainContextAfterAdd.Conversation.ID,
	))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rooms WHERE id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM members WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, mainRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 1, []string{mainContextAfterAdd.Conversation.ID})

	calls := coordinator.Calls()
	if len(calls) != 6 {
		t.Fatalf("应通过统一协调器清理 6 个 Room 成员 Session，实际 %+v", calls)
	}
	var foundSDKCleanup bool
	for _, call := range calls {
		if call.ownerUserID != mainContextAfterAdd.Room.OwnerUserID ||
			call.workspacePath == "" ||
			call.sessionKey == "" {
			t.Fatalf("Room Session artifact 删除参数不完整: %+v", call)
		}
		if call.sessionKey == topicAgentASession {
			foundSDKCleanup = call.cleanupSessionID == "sdk-topic-agent-a"
		}
	}
	if !foundSDKCleanup {
		t.Fatalf("Room 未把 SQL SDK session_id 交给统一协调器: %+v", calls)
	}
}

func TestRoomSessionArtifactCleanupFailsClosedWithoutCoordinator(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "保留助手A")
	agentB := createTestAgent(t, agentService, ctx, "保留助手B")
	agentC := createTestAgent(t, agentService, ctx, "待移除助手")
	contextValue, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID, agentC.AgentID},
		Name:     "缺失协调器测试",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	sessionKey := seedRoomPrivateSession(
		t,
		files,
		agentC.WorkspacePath,
		contextValue.Room.RoomType,
		contextValue.Conversation.ID,
		agentC.AgentID,
	)

	_, err = roomService.RemoveRoomMember(ctx, contextValue.Room.ID, agentC.AgentID)
	if !errors.Is(err, roomsvc.ErrSessionArtifactDeletionCoordinatorUnavailable) {
		t.Fatalf("缺少协调器必须 fail closed，实际 err=%v", err)
	}
	item, _, findErr := files.FindSession([]string{agentC.WorkspacePath}, sessionKey)
	if findErr != nil {
		t.Fatalf("核对 Session artifact 失败: %v", findErr)
	}
	if item == nil {
		t.Fatal("缺少协调器时不得回退为直接删除")
	}
}
