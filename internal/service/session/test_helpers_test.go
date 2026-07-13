package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func seedWorkspaceSessionArtifacts(
	t *testing.T,
	cfg config.Config,
	workspacePath string,
	sessionKey string,
	sessionID string,
) {
	t.Helper()

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round_1", "你好", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T10:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "msg_assistant_1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "最终回复"},
				},
			},
		},
	})
}

func seedRoomConversationMessages(t *testing.T, cfg config.Config, conversationID string) {
	t.Helper()

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err := roomHistory.AppendInlineMessage(conversationID, map[string]any{
		"message_id":      "room_msg_1",
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"conversation_id": conversationID,
		"agent_id":        "agent_room",
		"round_id":        "room_round_1",
		"role":            "assistant",
		"content":         "Room 共享消息",
		"timestamp":       100,
	}); err != nil {
		t.Fatalf("写入 room 共享历史失败: %v", err)
	}
}

func bindTranscriptSessionID(
	t *testing.T,
	cfg config.Config,
	workspacePath string,
	item *protocol.Session,
) string {
	t.Helper()

	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	item.SessionID = &sessionID
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(workspacePath, *item); err != nil {
		t.Fatalf("回写 session_id 失败: %v", err)
	}
	return sessionID
}

func findSessionByKey(items []protocol.Session, sessionKey string) *protocol.Session {
	for index := range items {
		if items[index].SessionKey == sessionKey {
			return &items[index]
		}
	}
	return nil
}

type staticTitleProvider struct {
	baseURL string
}

func (p staticTitleProvider) ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error) {
	return &clientopts.RuntimeConfig{
		Provider:  "test-title-provider",
		AuthToken: "token",
		BaseURL:   p.baseURL,
		Model:     "title-model",
	}, nil
}

func newSessionTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-session-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

var sessionTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func writeSessionTranscriptFixture(t *testing.T, workspacePath string, sessionID string, rows []map[string]any) {
	t.Helper()
	projectDir := sessionTranscriptProjectDir(workspacePath)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

func sessionTranscriptProjectDir(workspacePath string) string {
	return filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeSessionTranscriptPath(canonicalizeSessionTranscriptPath(workspacePath)),
	)
}

func sessionTranscriptFilePath(workspacePath string, sessionID string) string {
	return filepath.Join(sessionTranscriptProjectDir(workspacePath), strings.TrimSpace(sessionID)+".jsonl")
}

func canonicalizeSessionTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if absolutePath, err := filepath.Abs(path); err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeSessionTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := sessionTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + sessionTranscriptHash(path)
}

func sessionTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func stringValue(value any) string {
	typed, _ := value.(string)
	return typed
}

func stringPointer(value string) *string {
	copyValue := value
	return &copyValue
}

func migrateSessionSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, sessionMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func sessionMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
