package room_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type fakeRoomSkillCatalog map[string]skillspkg.Detail

func (f fakeRoomSkillCatalog) GetSkillDetail(_ context.Context, skillName string, _ string) (*skillspkg.Detail, error) {
	detail, ok := f[skillName]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &detail, nil
}

func findConversationContext(
	contexts []protocol.ConversationContextAggregate,
	conversationID string,
) (protocol.ConversationContextAggregate, bool) {
	for _, item := range contexts {
		if item.Conversation.ID == conversationID {
			return item, true
		}
	}
	return protocol.ConversationContextAggregate{}, false
}

type fakeRoomGoalCleaner struct {
	conversationCalls [][]string
	memberCalls       []fakeRoomGoalMemberCleanup
}

type fakeRoomGoalMemberCleanup struct {
	agentID         string
	conversationIDs []string
}

type fakeRoomRuntimeCloser struct {
	keys []string
}

func (f *fakeRoomRuntimeCloser) CloseSession(_ context.Context, sessionKey string) error {
	f.keys = append(f.keys, sessionKey)
	return nil
}

func (f *fakeRoomGoalCleaner) DeleteGoalsForRoomConversations(_ context.Context, conversationIDs []string) (int, error) {
	f.conversationCalls = append(f.conversationCalls, append([]string(nil), conversationIDs...))
	return len(conversationIDs), nil
}

func (f *fakeRoomGoalCleaner) DeleteGoalsForRoomMember(_ context.Context, agentID string, conversationIDs []string) (int, error) {
	f.memberCalls = append(f.memberCalls, fakeRoomGoalMemberCleanup{
		agentID:         agentID,
		conversationIDs: append([]string(nil), conversationIDs...),
	})
	return len(conversationIDs), nil
}

func createTestAgent(
	t *testing.T,
	service *agentsvc.Service,
	ctx context.Context,
	name string,
) *protocol.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	return item
}

func newRoomTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18011,
		ProjectName:    "nexus-room-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func seedRoomPrivateSession(
	t *testing.T,
	files *workspacestore.SessionFileStore,
	workspacePath string,
	roomType string,
	conversationID string,
	agentID string,
) string {
	t.Helper()

	sessionKey := protocol.BuildRoomAgentSessionKey(conversationID, agentID, roomType)
	now := time.Now().UTC()
	if _, err := files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:     sessionKey,
		AgentID:        agentID,
		ChannelType:    "websocket",
		ChatType:       "group",
		Status:         "active",
		CreatedAt:      now,
		LastActivity:   now,
		Title:          "Room Chat",
		MessageCount:   0,
		Options:        map[string]any{},
		IsActive:       true,
		ConversationID: stringPointer(conversationID),
	}); err != nil {
		t.Fatalf("创建 room 私有会话失败: %v", err)
	}
	return sessionKey
}

func seedRoomConversationLog(
	t *testing.T,
	root string,
	conversationID string,
	roomID string,
) {
	t.Helper()

	roomHistory := workspacestore.NewRoomHistoryStore(root)
	if err := roomHistory.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":      "seed_" + conversationID,
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"room_id":         roomID,
		"conversation_id": conversationID,
		"round_id":        "seed-round",
		"role":            "user",
		"content":         "seed",
		"timestamp":       time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("写入 room 共享日志失败: %v", err)
	}
}

func seedRoomDatabaseMessageRound(
	t *testing.T,
	db *sql.DB,
	conversationID string,
	sessionID string,
	suffix string,
) (string, string) {
	t.Helper()

	messageID := "msg-" + suffix
	roundID := "round-" + suffix
	if _, err := db.Exec(`
INSERT INTO messages (
    id, conversation_id, session_id, sender_type, kind, status,
    content_preview, jsonl_path, round_id
) VALUES (?, ?, ?, 'agent', 'text', 'completed', 'seed', 'seed.jsonl', ?)`,
		messageID,
		conversationID,
		sessionID,
		roundID,
	); err != nil {
		t.Fatalf("写入测试 message 失败: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO rounds (
    id, session_id, round_id, trigger_message_id, status
) VALUES (?, ?, ?, ?, 'success')`,
		"round-row-"+suffix,
		sessionID,
		roundID,
		messageID,
	); err != nil {
		t.Fatalf("写入测试 round 失败: %v", err)
	}
	return messageID, roundID
}

func findRoomSessionID(
	t *testing.T,
	contextValue protocol.ConversationContextAggregate,
	agentID string,
) string {
	t.Helper()

	for _, sessionValue := range contextValue.Sessions {
		if sessionValue.AgentID == agentID {
			return sessionValue.ID
		}
	}
	t.Fatalf("未找到 agent session: conversation=%s agent=%s", contextValue.Conversation.ID, agentID)
	return ""
}

func assertSQLCount(t *testing.T, db *sql.DB, query string, want int, args ...any) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("查询数量失败: %v query=%s", err, query)
	}
	if got != want {
		t.Fatalf("数量不符合预期: got=%d want=%d query=%s args=%v", got, want, query, args)
	}
}

func assertRoomGoalConversationCleanup(t *testing.T, cleaner *fakeRoomGoalCleaner, index int, wantConversationIDs []string) {
	t.Helper()
	if len(cleaner.conversationCalls) <= index {
		t.Fatalf("goal conversation cleanup calls = %#v, want index %d", cleaner.conversationCalls, index)
	}
	if !sameStringSet(cleaner.conversationCalls[index], wantConversationIDs) {
		t.Fatalf("goal conversation cleanup[%d] = %#v, want %#v", index, cleaner.conversationCalls[index], wantConversationIDs)
	}
}

func assertRoomGoalMemberCleanup(t *testing.T, cleaner *fakeRoomGoalCleaner, index int, wantAgentID string, wantConversationIDs []string) {
	t.Helper()
	if len(cleaner.memberCalls) <= index {
		t.Fatalf("goal member cleanup calls = %#v, want index %d", cleaner.memberCalls, index)
	}
	call := cleaner.memberCalls[index]
	if call.agentID != wantAgentID || !sameStringSet(call.conversationIDs, wantConversationIDs) {
		t.Fatalf("goal member cleanup[%d] = %#v, want agent=%s conversations=%#v", index, call, wantAgentID, wantConversationIDs)
	}
}

func assertRuntimeClosedKeys(t *testing.T, got []string, want []string) {
	t.Helper()
	if !sameStringSet(got, want) {
		t.Fatalf("runtime close keys = %#v, want %#v", got, want)
	}
}

func sameStringSet(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(want))
	for _, item := range got {
		counts[item]++
	}
	for _, item := range want {
		if counts[item] == 0 {
			return false
		}
		counts[item]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("期望路径存在: %s err=%v", path, err)
	}
}

func assertPathRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("期望路径已删除: %s err=%v", path, err)
	}
}

func migrateRoomSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, roomMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func roomMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
