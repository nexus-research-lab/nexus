package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

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
	if err := roomHistory.AppendInlineMessage("", conversationID, map[string]any{
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
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
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

func newSessionTestAgentService(t *testing.T, cfg config.Config) (*agentsvc.Service, *sql.DB) {
	t.Helper()

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭测试数据库失败: %v", err)
		}
	})
	return agentService, db
}

func newSessionTestCoreServices(
	t *testing.T,
	cfg config.Config,
) (*serverapp.CoreServices, *sql.DB) {
	t.Helper()
	db, err := serverapp.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭测试数据库失败: %v", err)
		}
	})
	return serverapp.NewCoreServicesWithDB(cfg, db), db
}

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
		workspacestore.TranscriptProjectsDirForWorkspace(workspacePath),
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
}

func sessionTranscriptFilePath(workspacePath string, sessionID string) string {
	return filepath.Join(sessionTranscriptProjectDir(workspacePath), strings.TrimSpace(sessionID)+".jsonl")
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
	handlertest.MigrateSQLiteFromDir(t, databaseURL, sessionMigrationDir(t))
}

func sessionMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
