package room_test

import (
	"context"
	"errors"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type failingRoomDeletionRuntime struct {
	err error
}

func (f failingRoomDeletionRuntime) CloseSession(context.Context, string) error {
	return f.err
}

func TestRoomSessionMaterializationClearsPendingForkDependency(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 Agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "fork 物化助手")
	created, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "fork 物化 room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	roomSessionID := findRoomSessionID(t, *created, agentValue.AgentID)
	if _, err = db.ExecContext(ctx, `
UPDATE sessions
SET options_json = ?
WHERE id = ?`, `{"runtime_fork_source_session_id":"source-sdk","runtime_fork_message_id":"source-message"}`, roomSessionID); err != nil {
		t.Fatalf("写入 pending fork 依赖失败: %v", err)
	}

	if err = roomService.UpdateSessionRuntimeIdentity(ctx, roomSessionID, "target-sdk", ""); err != nil {
		t.Fatalf("物化 target SDK session 失败: %v", err)
	}
	contextValue, err := roomService.GetConversationContext(ctx, created.Conversation.ID)
	if err != nil {
		t.Fatalf("读取物化后 Session 失败: %v", err)
	}
	sessionValue := contextValue.Sessions[0]
	if _, exists := sessionValue.Options[protocol.OptionRuntimeForkSourceSessionID]; exists {
		t.Fatalf("物化后仍保留 fork source 依赖: %+v", sessionValue.Options)
	}
	if _, exists := sessionValue.Options[protocol.OptionRuntimeForkMessageID]; exists {
		t.Fatalf("物化后仍保留 fork message 依赖: %+v", sessionValue.Options)
	}
	if sessionValue.SDKSessionID != "target-sdk" ||
		!sameStringSet(sessionValue.TranscriptSessionIDs, []string{"target-sdk"}) {
		t.Fatalf("物化后 transcript lineage 不正确: %+v", sessionValue)
	}
	if retained := protocol.RetainedTranscriptSessionIDsFromOptions(sessionValue.Options); !sameStringSet(retained, []string{"source-sdk"}) {
		t.Fatalf("物化后未转移 source transcript 清理所有权: %+v", sessionValue.Options)
	}
	if cleanupIDs := protocol.RoomSessionCleanupTranscriptIDs(sessionValue); !sameStringSet(cleanupIDs, []string{"source-sdk", "target-sdk"}) {
		t.Fatalf("物化后 transcript 清理集合不正确: %+v", cleanupIDs)
	}
}

func TestRoomSessionKeepsTranscriptLineageAcrossSDKIdentityChanges(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 Agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "lineage 助手")
	created, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "lineage room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	roomSessionID := findRoomSessionID(t, *created, agentValue.AgentID)
	firstID := "550e8400-e29b-41d4-a716-446655440000"
	secondID := "650e8400-e29b-41d4-a716-446655440000"
	for _, sdkSessionID := range []string{firstID, secondID, ""} {
		if err = roomService.UpdateSessionRuntimeIdentity(ctx, roomSessionID, sdkSessionID, ""); err != nil {
			t.Fatalf("更新 SDK session id %q 失败: %v", sdkSessionID, err)
		}
	}
	contexts, err := roomService.GetRoomContexts(ctx, created.Room.ID)
	if err != nil {
		t.Fatalf("读取 Room contexts 失败: %v", err)
	}
	if len(contexts) != 1 || len(contexts[0].Sessions) != 1 {
		t.Fatalf("Room Session 投影错误: %+v", contexts)
	}
	sessionValue := contexts[0].Sessions[0]
	if sessionValue.SDKSessionID != "" ||
		!sameStringSet(sessionValue.TranscriptSessionIDs, []string{firstID, secondID}) {
		t.Fatalf("SDK identity 清空后 lineage 丢失: %+v", sessionValue)
	}
	if err = roomService.DeleteRoom(ctx, created.Room.ID); err != nil {
		t.Fatalf("删除 Room 失败: %v", err)
	}
	assertSQLCount(t, db, "SELECT COUNT(*) FROM sessions WHERE id = ?", 0, roomSessionID)
}

func TestRoomDeletionCanRetryWhileSourceRecordRemains(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 Agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "删除恢复助手")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "删除恢复 room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "待恢复删除"},
	)
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	runtimeFailure := errors.New("runtime close unavailable")
	roomService.SetRuntimeManager(failingRoomDeletionRuntime{err: runtimeFailure})
	if _, err = roomService.DeleteConversation(
		ctx,
		mainContext.Room.ID,
		topicContext.Conversation.ID,
	); !errors.Is(err, runtimeFailure) {
		t.Fatalf("首次删除应保留 runtime 失败: %v", err)
	}
	assertSQLCount(t, db, "SELECT COUNT(*) FROM conversations WHERE id = ?", 1, topicContext.Conversation.ID)

	roomService.SetRuntimeManager(&fakeRoomRuntimeCloser{})
	fallback, err := roomService.DeleteConversation(
		ctx,
		mainContext.Room.ID,
		topicContext.Conversation.ID,
	)
	if err != nil {
		t.Fatalf("重试删除失败: %v", err)
	}
	if fallback == nil || fallback.Conversation.ID != mainContext.Conversation.ID {
		t.Fatalf("重试删除回退上下文错误: %+v", fallback)
	}
	assertSQLCount(t, db, "SELECT COUNT(*) FROM conversations WHERE id = ?", 0, topicContext.Conversation.ID)
}
