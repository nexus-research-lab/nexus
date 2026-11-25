package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithMinimalAppRoot(m))
}

// accelerateDMGoalUsageRetry 缩短白盒测试中的退避时钟，不改变生产默认值。
func accelerateDMGoalUsageRetry(runner *roundRunner) {
	if runner != nil {
		runner.goalUsageRetryBaseDelay = time.Millisecond
	}
}

// accelerateDMExternalTyping 缩短白盒测试中的 typing 启动等待，不改变生产默认值。
func accelerateDMExternalTyping(runner *roundRunner) {
	if runner != nil {
		runner.externalTypingDelay = 5 * time.Millisecond
	}
}

func newDMAgentService(t *testing.T, cfg config.Config) *agentsvc.Service {
	t.Helper()
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
}

// newPlainDMAgent 创建普通 Agent；主智能体的 DM 是主窗口，输入走 inbox，测旧待发送队列要用普通 Agent。
func newPlainDMAgent(t *testing.T, agentService *agentsvc.Service) *protocol.Agent {
	t.Helper()
	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "Queue Agent"})
	if err != nil {
		t.Fatalf("创建普通测试 Agent 失败: %v", err)
	}
	return created
}

// newDMInboxStore 打开与测试 Agent 相同的数据库；主智能体 DM 的输入都经过 inbox。
func newDMInboxStore(t *testing.T, cfg config.Config) *inboxstore.Repository {
	t.Helper()
	// 与生产一致带 busy_timeout：inbox 与测试 Agent 仓储是两个连接池，会并发写同一个 SQLite 文件。
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开 inbox 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return inboxstore.NewRepository(cfg, db)
}

func newDMProviderService(t *testing.T, cfg config.Config) *providercfg.Service {
	t.Helper()
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 provider 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return providercfg.NewServiceWithDB(cfg, db)
}

func createDMProviderWithModel(
	t *testing.T,
	service *providercfg.Service,
	input providercfg.CreateInput,
	model string,
	isDefault bool,
) *providercfg.Record {
	t.Helper()
	record, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(context.Background(), record.Provider, model, providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: isDefault,
	}); err != nil {
		t.Fatalf("设置 provider 模型失败: %v", err)
	}
	return record
}

func newDMTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-dm-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func mustFindDMSession(
	t *testing.T,
	service *Service,
	cfg config.Config,
	sessionKey string,
) (protocol.Session, string) {
	t.Helper()
	workspacePath := dmMainWorkspacePath(cfg)
	if agentID := protocol.ParseSessionKey(sessionKey).AgentID; agentID != cfg.DefaultAgentID {
		agentValue, err := service.Agents.GetAgent(context.Background(), agentID)
		if err != nil {
			t.Fatalf("读取 session 所属 Agent 失败: %v", err)
		}
		workspacePath = agentValue.WorkspacePath
	}
	item, workspacePath, err := service.Files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatalf("session 元数据不存在: %s", sessionKey)
	}
	return *item, workspacePath
}

func dmMainWorkspacePath(cfg config.Config) string {
	return agentsvc.ResolveWorkspacePath(cfg, authctx.SystemUserID, cfg.DefaultAgentID)
}

func readDMSessionHistory(
	t *testing.T,
	cfg config.Config,
	service *Service,
	sessionKey string,
) []protocol.Message {
	t.Helper()
	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	historyStore := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	rows, err := historyStore.ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		t.Fatalf("读取 transcript 历史失败: %v", err)
	}
	return rows
}

func writeTranscriptFixture(
	t *testing.T,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	writeTranscriptFixtureAt(
		t,
		workspacestore.TranscriptProjectsDirForWorkspace(workspacePath),
		workspacePath,
		sessionID,
		rows,
	)
}

func writeOwnerTranscriptFixture(
	t *testing.T,
	ownerUserID string,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	writeTranscriptFixtureAt(
		t,
		filepath.Join(appfs.UserRuntimeRoot(ownerUserID), "projects"),
		workspacePath,
		sessionID,
		rows,
	)
}

func writeTranscriptFixtureAt(
	t *testing.T,
	projectsRoot string,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 transcript fixture")
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("创建 workspace 目录失败: %v", err)
	}
	projectDir := filepath.Join(
		projectsRoot,
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
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

func stringPointer(t *testing.T, value *string) string {
	t.Helper()
	if value == nil || strings.TrimSpace(*value) == "" {
		t.Fatal("session_id 未持久化")
	}
	return strings.TrimSpace(*value)
}

func migrateDMSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, dmMigrationDir(t))
}

func dmMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
