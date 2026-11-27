// domain_migrations_test.go 覆盖凭据、会话草稿修复、Provider 作用域恢复与废弃资源迁移测试。
package migration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const (
	migrationActiveKey  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	migrationLegacyKey  = "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODk="
	migrationUnknownKey = "enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU="
)

func TestRunConnectorCredentialKeyMigrationResumesMixedKeyRows(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "connector-keyring.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}

	activeKey := decodeMigrationKey(t, migrationActiveKey)
	legacyKey := decodeMigrationKey(t, migrationLegacyKey)
	unknownKey := decodeMigrationKey(t, migrationUnknownKey)
	activeKeyID := credentials.KeyID(activeKey)
	legacyKeyID := credentials.KeyID(legacyKey)
	unknownEncrypted := insertMigrationCredential(t, db, "unknown", unknownKey, "", "unknown")
	insertMigrationCredential(t, db, "active-current", activeKey, activeKeyID, "active-current")
	insertMigrationCredential(t, db, "active-unidentified", activeKey, "", "active-unidentified")
	insertMigrationCredential(t, db, "legacy-unidentified", legacyKey, "", "legacy-unidentified")
	insertMigrationCredential(t, db, "legacy-identified", legacyKey, legacyKeyID, "legacy-identified")
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', 'custom-mcp:corrupt', 'connected', TRUE, '__encrypted__', 'v1:not-valid', ?, 'custom_mcp')`, legacyKeyID); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		DatabaseDriver:                 "sqlite",
		DatabaseURL:                    databaseURL,
		ConnectorCredentialsKey:        migrationActiveKey,
		ConnectorCredentialsLegacyKeys: []string{migrationLegacyKey},
	}
	report, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 6 || report.AlreadyCurrent != 1 || report.Identified != 1 ||
		report.Reencrypted != 2 || report.RecoveryRequired != 1 || report.Corrupt != 1 ||
		report.Conflicted != 0 || !report.KeyringAvailable {
		t.Fatalf("first migration report mismatch: %+v", report)
	}

	db, err = sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, connectorID := range []string{
		"active-current",
		"active-unidentified",
		"legacy-unidentified",
		"legacy-identified",
	} {
		var encrypted string
		var keyID sql.NullString
		if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = ?`, "custom-mcp:"+connectorID).Scan(&encrypted, &keyID); err != nil {
			t.Fatal(err)
		}
		plain, decryptErr := credentials.DecryptPayload(activeKey, encrypted)
		if decryptErr != nil || string(plain) != connectorID || !keyID.Valid || keyID.String != activeKeyID {
			t.Fatalf("%s was not migrated to active key: plain=%q key_id=%q err=%v", connectorID, plain, keyID.String, decryptErr)
		}
	}
	var unknownAfter string
	var unknownKeyID sql.NullString
	if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = 'custom-mcp:unknown'`).Scan(&unknownAfter, &unknownKeyID); err != nil {
		t.Fatal(err)
	}
	if unknownAfter != unknownEncrypted || unknownKeyID.Valid {
		t.Fatalf("unknown-key row must remain untouched: key_id=%q", unknownKeyID.String)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	repeated, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if repeated.AlreadyCurrent != 4 || repeated.Identified != 0 || repeated.Reencrypted != 0 ||
		repeated.RecoveryRequired != 1 || repeated.Corrupt != 1 {
		t.Fatalf("repeated migration must resume idempotently: %+v", repeated)
	}
}

func decodeMigrationKey(t *testing.T, raw string) []byte {
	t.Helper()
	key, err := credentials.DecodeKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func insertMigrationCredential(
	t *testing.T,
	db *sql.DB,
	suffix string,
	key []byte,
	keyID string,
	plain string,
) string {
	t.Helper()
	encrypted, err := credentials.EncryptPayload(key, []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	var storedKeyID any
	if keyID != "" {
		storedKeyID = keyID
	}
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', ?, 'connected', TRUE, '__encrypted__', ?, ?, 'custom_mcp')`,
		"custom-mcp:"+suffix,
		encrypted,
		storedKeyID,
	); err != nil {
		t.Fatal(err)
	}
	return encrypted
}

func TestDesktopLegacyConversationDraftRepairStartedMarkerSuppressesRetry(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "repair.marker")
	cfg := config.Config{AppMode: "desktop", DatabaseDriver: "sqlite"}
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	calls := 0
	firstFailure := errors.New("simulated repair failure")
	apply := func(
		context.Context,
		config.Config,
		*slog.Logger,
	) (legacyConversationDraftRepairSummary, error) {
		calls++
		return legacyConversationDraftRepairSummary{}, firstFailure
	}

	err := runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	)
	if !errors.Is(err, firstFailure) {
		t.Fatalf("首次失败 = %v, want wrapped simulated failure", err)
	}
	assertOneTimeRepairMarker(t, markerPath, oneTimeRepairStateStarted)

	if err = runDesktopLegacyConversationDraftRepairOnce(
		t.Context(),
		cfg,
		logger,
		markerPath,
		apply,
	); err != nil {
		t.Fatalf("started marker 应安全跳过自动重跑: %v", err)
	}
	if calls != 1 {
		t.Fatalf("started marker 后不应重跑，apply 次数 = %d", calls)
	}
	if !strings.Contains(logOutput.String(), "跳过自动重跑") {
		t.Fatalf("started marker 应记录维护提示: %s", logOutput.String())
	}
}

func TestDesktopLegacyConversationDraftRepairSkipsUnsafeDeployments(t *testing.T) {
	for name, cfg := range map[string]config.Config{
		"sqlite server": {
			AppMode:        "server",
			DatabaseDriver: "sqlite",
		},
		"postgres desktop": {
			AppMode:        "desktop",
			DatabaseDriver: "postgres",
		},
	} {
		t.Run(name, func(t *testing.T) {
			markerPath := filepath.Join(t.TempDir(), "repair.marker")
			called := false
			err := runDesktopLegacyConversationDraftRepairOnce(
				t.Context(),
				cfg,
				discardMigrationLogger(),
				markerPath,
				func(
					context.Context,
					config.Config,
					*slog.Logger,
				) (legacyConversationDraftRepairSummary, error) {
					called = true
					return legacyConversationDraftRepairSummary{}, nil
				},
			)
			if err != nil {
				t.Fatalf("不安全部署应直接跳过: %v", err)
			}
			if called {
				t.Fatal("不安全部署不应执行 apply")
			}
			if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
				t.Fatalf("跳过时不应创建 marker: %v", statErr)
			}
		})
	}
}

func TestDesktopLegacyConversationDraftRepairCoversEveryRoomOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	cfg := config.Config{
		AppMode:        "desktop",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(stateRoot, "app", "data", "nexus.db"),
		WorkspacePath:  filepath.Join(stateRoot, "users"),
		GoalEnabled:    true,
	}
	migrateConversationDraftRepairDatabase(t, cfg)

	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库: %v", err)
	}
	agentService := agentsvc.NewService(
		cfg,
		agentrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	roomService := roomsvc.NewService(
		cfg,
		agentService,
		roomrepo.NewSQLRepository(cfg.DatabaseDriver, db),
	)
	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	type fixture struct {
		ownerID      string
		roomID       string
		occupiedID   string
		olderEmptyID string
		keeperID     string
	}
	fixtures := make([]fixture, 0, 2)
	for index, ownerID := range []string{"owner-a", "owner-b"} {
		ownerContext := authctx.WithPrincipal(t.Context(), &authctx.Principal{
			UserID:     ownerID,
			Username:   ownerID,
			Role:       authctx.RoleOwner,
			AuthMethod: "test",
		})
		agentValue, createErr := agentService.CreateAgent(ownerContext, protocol.CreateRequest{
			Name: "迁移测试助手 " + ownerID,
		})
		if createErr != nil {
			t.Fatalf("owner %s 创建 Agent: %v", ownerID, createErr)
		}
		mainContext, createErr := roomService.CreateRoom(ownerContext, protocol.CreateRoomRequest{
			AgentIDs: []string{agentValue.AgentID},
			Name:     "迁移测试 Room " + ownerID,
		})
		if createErr != nil {
			t.Fatalf("owner %s 创建 Room: %v", ownerID, createErr)
		}
		if _, createErr = db.ExecContext(
			t.Context(),
			`UPDATE conversations SET is_draft = 0 WHERE room_id = ?`,
			mainContext.Room.ID,
		); createErr != nil {
			t.Fatalf("owner %s 准备旧版本主会话: %v", ownerID, createErr)
		}
		olderEmpty, createErr := roomService.CreateConversation(
			ownerContext,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: "可能已生成过的标题"},
		)
		if createErr != nil {
			t.Fatalf("owner %s 创建旧空白 Session: %v", ownerID, createErr)
		}
		if _, createErr = db.ExecContext(
			t.Context(),
			`UPDATE conversations SET is_draft = 0 WHERE room_id = ?`,
			mainContext.Room.ID,
		); createErr != nil {
			t.Fatalf("owner %s 准备旧版本空白会话: %v", ownerID, createErr)
		}
		keeper, createErr := roomService.CreateConversation(
			ownerContext,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: "标题不能代替用户输入证据"},
		)
		if createErr != nil {
			t.Fatalf("owner %s 创建新空白 Session: %v", ownerID, createErr)
		}
		if err = history.AppendInlineMessage(ownerID, mainContext.Conversation.ID, protocol.Message{
			"message_id": "user-" + ownerID,
			"role":       "user",
			"content":    "已经输入过",
			"timestamp":  int64(index + 1),
		}); err != nil {
			t.Fatalf("owner %s 写入 canonical 用户输入: %v", ownerID, err)
		}
		if _, err = db.ExecContext(t.Context(), `
UPDATE conversations
SET is_draft = 0,
    created_at = CASE id
        WHEN ? THEN '2026-01-01 09:00:00'
        WHEN ? THEN '2026-01-01 10:00:00'
        ELSE '2026-01-01 11:00:00'
    END,
    updated_at = created_at,
    last_activity_at = created_at
WHERE room_id = ?`,
			mainContext.Conversation.ID,
			olderEmpty.Conversation.ID,
			mainContext.Room.ID,
		); err != nil {
			t.Fatalf("owner %s 准备旧版本 fixture: %v", ownerID, err)
		}
		fixtures = append(fixtures, fixture{
			ownerID:      ownerID,
			roomID:       mainContext.Room.ID,
			occupiedID:   mainContext.Conversation.ID,
			olderEmptyID: olderEmpty.Conversation.ID,
			keeperID:     keeper.Conversation.ID,
		})
	}
	if err = db.Close(); err != nil {
		t.Fatalf("关闭 fixture 数据库: %v", err)
	}

	if err = RunDesktopLegacyConversationDraftRepair(
		t.Context(),
		cfg,
		discardMigrationLogger(),
	); err != nil {
		t.Fatalf("执行桌面升级修复: %v", err)
	}
	assertOneTimeRepairMarker(
		t,
		legacyConversationDraftRepairMarkerPath(),
		oneTimeRepairStateCompleted,
	)

	db, err = storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("重新打开测试数据库: %v", err)
	}
	defer db.Close()
	for _, item := range fixtures {
		var (
			conversationCount int
			draftCount        int
			keeperDraft       bool
			occupiedCount     int
			olderEmptyCount   int
		)
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*), SUM(CASE WHEN is_draft THEN 1 ELSE 0 END)
FROM conversations WHERE room_id = ?`,
			item.roomID,
		).Scan(&conversationCount, &draftCount); err != nil {
			t.Fatalf("owner %s 读取收口结果: %v", item.ownerID, err)
		}
		if conversationCount != 2 || draftCount != 1 {
			t.Fatalf(
				"owner %s 收口结果 = conversations:%d drafts:%d, want 2/1",
				item.ownerID,
				conversationCount,
				draftCount,
			)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT is_draft FROM conversations WHERE id = ?`,
			item.keeperID,
		).Scan(&keeperDraft); err != nil || !keeperDraft {
			t.Fatalf("owner %s keeper 必须成为 draft: draft=%v err=%v", item.ownerID, keeperDraft, err)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM conversations WHERE id = ?`,
			item.occupiedID,
		).Scan(&occupiedCount); err != nil || occupiedCount != 1 {
			t.Fatalf("owner %s 有用户输入的 Session 必须保留: count=%d err=%v", item.ownerID, occupiedCount, err)
		}
		if err = db.QueryRowContext(
			t.Context(),
			`SELECT COUNT(*) FROM conversations WHERE id = ?`,
			item.olderEmptyID,
		).Scan(&olderEmptyCount); err != nil || olderEmptyCount != 0 {
			t.Fatalf("owner %s 旧空白 Session 必须删除: count=%d err=%v", item.ownerID, olderEmptyCount, err)
		}
	}
}

func migrateConversationDraftRepairDatabase(t *testing.T, cfg config.Config) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开 migration 数据库: %v", err)
	}
	defer db.Close()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言: %v", err)
	}
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatalf("执行基础 migration: %v", err)
	}
}

func assertOneTimeRepairMarker(t *testing.T, markerPath string, expected string) {
	t.Helper()
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("读取 repair marker %q: %v", markerPath, err)
	}
	if string(content) != expected+"\n" {
		t.Fatalf("repair marker %q = %q, want %q", markerPath, content, expected+"\n")
	}
	info, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("读取 repair marker 权限: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("repair marker 权限 = %o, want 600", info.Mode().Perm())
	}
}

func TestListRoomOwnerUserIDsRejectsUnownedRoom(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "owners.db"),
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开 owners 数据库: %v", err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE rooms (owner_user_id TEXT)`); err != nil {
		t.Fatalf("创建 rooms fixture: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO rooms (owner_user_id) VALUES ('owner-b'), ('owner-a'), ('owner-a')`); err != nil {
		t.Fatalf("插入 owner fixture: %v", err)
	}
	owners, err := listRoomOwnerUserIDs(t.Context(), db)
	if err != nil {
		t.Fatalf("读取 owner: %v", err)
	}
	if strings.Join(owners, ",") != "owner-a,owner-b" {
		t.Fatalf("owner 列表 = %v", owners)
	}
	if _, err = db.Exec(`INSERT INTO rooms (owner_user_id) VALUES ('')`); err != nil {
		t.Fatalf("插入无 owner fixture: %v", err)
	}
	if _, err = listRoomOwnerUserIDs(t.Context(), db); err == nil {
		t.Fatal("缺少 owner_user_id 时必须停止自动修复")
	}
}

func TestRepairDesktopProviderScopeSkipsWebDeployment(t *testing.T) {
	for name, cfg := range map[string]config.Config{
		"postgres server": {
			AppMode:        "server",
			DatabaseDriver: "postgres",
			DatabaseURL:    "this-is-not-a-database-url",
		},
		"sqlite server": {
			AppMode:        "server",
			DatabaseDriver: "sqlite",
			DatabaseURL:    filepath.Join(t.TempDir(), "must-not-open.db"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := RepairDesktopProviderScope(t.Context(), cfg, discardMigrationLogger()); err != nil {
				t.Fatalf("Web 部署不应执行桌面 Provider 补偿: %v", err)
			}
			if cfg.DatabaseDriver == "sqlite" {
				if _, err := os.Stat(cfg.DatabaseURL); !os.IsNotExist(err) {
					t.Fatalf("Web 部署不应创建本地 Provider 修复数据库: stat err=%v", err)
				}
			}
		})
	}
}

func TestProviderScopeRecoveryCopiesLegacyProvidersPerRuntimeOwner(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "nexus.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := providerRecoveryMigrationDir(t)
	if err = goose.UpTo(db, dir, 49); err != nil {
		t.Fatalf("迁移到 Provider 修复前版本失败: %v", err)
	}

	insertRecoveryAgent(t, db, "agent-a", "owner-a", "legacy-provider")
	insertRecoveryAgent(t, db, "agent-b", "owner-b", "legacy-provider")
	insertRecoveryAgent(t, db, "agent-new", "owner-a", "late-public")
	insertRecoveryProvider(t, db, "legacy-provider-id", "legacy-provider", "2026-05-25 00:00:00")
	// 迁移 18 之后误建的无主公共 Provider：桌面端同样按事故恢复，不按创建时间豁免。
	insertRecoveryProvider(t, db, "late-public-id", "late-public", "2026-06-16 00:00:00")
	// 没有任何 runtime/preferences 引用的无主公共 Provider：走兜底归属。
	insertRecoveryProvider(t, db, "orphan-public-id", "orphan-public", "2026-06-20 00:00:00")
	if _, err = db.Exec(`
INSERT INTO provider_models (
    id, provider_id, model_id, display_name, category, enabled, is_default,
    capabilities_auto_json, capabilities_override_json, provider_options_json
) VALUES ('legacy-model-id', 'legacy-provider-id', 'legacy-model', 'Legacy Model', 'chat', 1, 1, '{}', '{}', '{}')`); err != nil {
		t.Fatalf("插入旧 Provider 模型失败: %v", err)
	}

	cfg := config.Config{
		AppMode:        "desktop",
		DatabaseDriver: "sqlite",
		DatabaseURL:    databaseURL,
	}
	if err = RepairDesktopProviderScope(t.Context(), cfg, discardMigrationLogger()); err != nil {
		t.Fatalf("执行桌面 Provider scope 补偿失败: %v", err)
	}

	var recoveryCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM provider_scope_recovery WHERE source_provider_id = 'legacy-provider-id'`).Scan(&recoveryCount); err != nil {
		t.Fatalf("读取 Provider 修复账本失败: %v", err)
	}
	if recoveryCount != 2 {
		t.Fatalf("修复账本数量 = %d, want 2", recoveryCount)
	}

	var privateCount int
	if err = db.QueryRow(`
SELECT COUNT(*)
FROM provider
WHERE provider = 'legacy-provider' AND visibility = 'private'`).Scan(&privateCount); err != nil {
		t.Fatalf("读取恢复后的私有 Provider 失败: %v", err)
	}
	if privateCount != 2 {
		t.Fatalf("恢复后的私有 Provider 数量 = %d, want 2", privateCount)
	}

	var copiedModelCount int
	if err = db.QueryRow(`
SELECT COUNT(*)
FROM provider_models models
JOIN provider providers ON providers.id = models.provider_id
WHERE providers.provider = 'legacy-provider' AND providers.visibility = 'private'`).Scan(&copiedModelCount); err != nil {
		t.Fatalf("读取恢复后的 Provider 模型失败: %v", err)
	}
	if copiedModelCount != 2 {
		t.Fatalf("恢复后的模型数量 = %d, want 2", copiedModelCount)
	}

	var legacyPublicCount int
	if err = db.QueryRow(`
SELECT COUNT(*)
FROM provider
WHERE id = 'legacy-provider-id' AND visibility = 'public'`).Scan(&legacyPublicCount); err != nil {
		t.Fatalf("读取旧公共 Provider 清理状态失败: %v", err)
	}
	if legacyPublicCount != 1 {
		t.Fatal("旧公共 Provider 应保留为安全 fallback")
	}

	var lateOwner string
	if err = db.QueryRow(`
SELECT owner_user_id
FROM provider
WHERE provider = 'late-public' AND visibility = 'private'`).Scan(&lateOwner); err != nil {
		t.Fatalf("读取迁移后误建 Provider 的恢复结果失败: %v", err)
	}
	if lateOwner != "owner-a" {
		t.Fatalf("迁移后误建 Provider 恢复 owner = %q, want owner-a", lateOwner)
	}

	var orphanOwner string
	if err = db.QueryRow(`
SELECT owner_user_id
FROM provider
WHERE provider = 'orphan-public' AND visibility = 'private'`).Scan(&orphanOwner); err != nil {
		t.Fatalf("读取兜底恢复结果失败: %v", err)
	}
	if orphanOwner != "__system__" {
		t.Fatalf("兜底恢复 owner = %q, want __system__", orphanOwner)
	}
	var orphanPublicCount int
	if err = db.QueryRow(`
SELECT COUNT(*) FROM provider WHERE id = 'orphan-public-id' AND visibility = 'public'`).Scan(&orphanPublicCount); err != nil {
		t.Fatalf("读取兜底 Provider 公共 fallback 失败: %v", err)
	}
	if orphanPublicCount != 1 {
		t.Fatal("兜底恢复后公共 Provider 应保留为 fallback")
	}

	if err = RepairDesktopProviderScope(t.Context(), cfg, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行桌面 Provider scope 补偿失败: %v", err)
	}
	var repeatedPrivateCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM provider WHERE visibility = 'private' AND provider = 'legacy-provider'`).Scan(&repeatedPrivateCount); err != nil {
		t.Fatalf("读取重复补偿后的 Provider 数量失败: %v", err)
	}
	if repeatedPrivateCount != 2 {
		t.Fatalf("重复补偿不应新增私有 Provider: got %d, want 2", repeatedPrivateCount)
	}
}

func TestRepairDesktopProviderScopeCopiesPreferenceProvider(t *testing.T) {
	root := t.TempDir()
	databaseURL := filepath.Join(root, "nexus.db")
	workspaceRoot := filepath.Join(root, "workspace")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := providerRecoveryMigrationDir(t)
	if err = goose.Up(db, dir); err != nil {
		t.Fatalf("执行基础 migration 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO users (user_id, username, display_name, role, status)
VALUES ('owner-a', 'owner-a', 'Owner A', 'owner', 'active')`); err != nil {
		t.Fatalf("插入测试用户失败: %v", err)
	}
	insertRecoveryProvider(t, db, "preference-provider-id", "preference-provider", "2026-05-25 00:00:00")
	if _, err = db.Exec(`
INSERT INTO provider_models (
    id, provider_id, model_id, display_name, category, enabled, is_default,
    capabilities_auto_json, capabilities_override_json, provider_options_json
) VALUES ('preference-model-id', 'preference-provider-id', 'preference-model', 'Preference Model', 'chat', 1, 1, '{}', '{}', '{}')`); err != nil {
		t.Fatalf("插入偏好 Provider 模型失败: %v", err)
	}
	preferencesPath := filepath.Join(
		workspaceRoot,
		"owner-a",
		"workspace",
		".settings",
		"preferences.json",
	)
	if err = os.MkdirAll(filepath.Dir(preferencesPath), 0o755); err != nil {
		t.Fatalf("创建偏好目录失败: %v", err)
	}
	preferences, _ := json.Marshal(map[string]any{
		"default_agent_options": map[string]string{"provider": "preference-provider"},
	})
	if err = os.WriteFile(preferencesPath, preferences, 0o644); err != nil {
		t.Fatalf("写入偏好文件失败: %v", err)
	}

	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    databaseURL,
		WorkspacePath:  workspaceRoot,
	}
	cfg.AppMode = "desktop"
	if err = RepairDesktopProviderScope(t.Context(), cfg, discardMigrationLogger()); err != nil {
		t.Fatalf("执行偏好 Provider 补偿迁移失败: %v", err)
	}

	var visibility string
	var ownerUserID string
	if err = db.QueryRow(`
SELECT visibility, owner_user_id
FROM provider
WHERE provider = 'preference-provider' AND visibility = 'private'`).Scan(&visibility, &ownerUserID); err != nil {
		t.Fatalf("读取偏好 Provider 修复结果失败: %v", err)
	}
	if visibility != "private" || ownerUserID != "owner-a" {
		t.Fatalf("偏好 Provider scope = (%q, %q), want (private, owner-a)", visibility, ownerUserID)
	}
	var modelCount int
	if err = db.QueryRow(`
SELECT COUNT(*)
FROM provider_models models
JOIN provider providers ON providers.id = models.provider_id
WHERE providers.provider = 'preference-provider' AND providers.visibility = 'private'`).Scan(&modelCount); err != nil {
		t.Fatalf("读取偏好 Provider 模型失败: %v", err)
	}
	if modelCount != 1 {
		t.Fatalf("偏好 Provider 模型数量 = %d, want 1", modelCount)
	}
}

func providerRecoveryMigrationDir(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

func insertRecoveryAgent(t *testing.T, db *sql.DB, agentID string, ownerUserID string, provider string) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', 'active', ?, ?, 0)`,
		agentID, agentID, agentID, "/tmp/"+agentID, ownerUserID); err != nil {
		t.Fatalf("插入测试 Agent 失败: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO runtimes (
    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
    mcp_servers_json, setting_sources_json, runtime_version
) VALUES (?, ?, ?, '', '[]', '[]', '{}', '[]', 1)`,
		"runtime-"+agentID, agentID, provider); err != nil {
		t.Fatalf("插入测试 runtime 失败: %v", err)
	}
}

func insertRecoveryProvider(t *testing.T, db *sql.DB, id string, provider string, createdAt string) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO provider (
    id, provider, display_name, auth_token, base_url, enabled,
    created_at, updated_at, provider_kind, preset_key, api_format, models_path,
    last_test_status, last_test_error, visibility
) VALUES (?, ?, ?, 'token', 'https://example.test', 1, ?, ?, 'llm', 'custom', 'anthropic_messages', '/v1/models', '', '', 'public')`,
		id, provider, provider, createdAt, createdAt); err != nil {
		t.Fatalf("插入测试 Provider 失败: %v", err)
	}
}

func TestRunWorkspaceFilesRemovesRetiredSandboxResources(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	workspaceRoot := filepath.Join(stateRoot, "workspace")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	readOnlyCache := filepath.Join(leasedLeaf, "gomod", "pkg@v1")
	unmarkedLeaf := filepath.Join(scratchBase, "not-ours")
	otherOwnerBase := filepath.Join(stateRoot, "users", "owner-b", "runtime", "sandbox")
	keptRuntimeFile := filepath.Join(stateRoot, "users", "owner-a", "runtime", "settings.json")

	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "p", "job.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "scratch-recovery", "q", "file"), "x\n")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(readOnlyCache, "go.mod"), "module x\n")
	writeMigrationTestFile(t, filepath.Join(unmarkedLeaf, "keep.txt"), "keep\n")
	writeMigrationTestFile(t, filepath.Join(otherOwnerBase, "lease-2", retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, keptRuntimeFile, "{}\n")
	if err := os.Chmod(readOnlyCache, 0o500); err != nil {
		t.Fatal(err)
	}

	if err := RunWorkspaceFiles(appRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行工作区文件迁移失败: %v", err)
	}
	assertMigrationPathMissing(t, filepath.Join(appRoot, "processes"))
	assertMigrationPathMissing(t, leasedLeaf)
	assertMigrationPathMissing(t, otherOwnerBase)
	assertMigrationPathExists(t, filepath.Join(unmarkedLeaf, "keep.txt"))
	assertMigrationPathExists(t, keptRuntimeFile)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

func TestRunWorkspaceFilesRetriesRetiredSandboxCleanupWithoutBlockingStartup(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	lockedDir := filepath.Join(leasedLeaf, "locked")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(lockedDir, "file"), "x\n")
	// 改名成功、删除中途失败：租约标记已被删掉，墓碑名仍须让下次启动识别。
	failRemove := true
	previousRemove := removeRetiredTree
	removeRetiredTree = func(root string) error {
		if failRemove {
			_ = os.Remove(filepath.Join(root, retiredSandboxLeaseMarkerName))
			return os.ErrPermission
		}
		return previousRemove(root)
	}
	t.Cleanup(func() { removeRetiredTree = previousRemove })

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatalf("尽力型清理失败不应阻断启动: %v", err)
	}
	assertMigrationPathExists(t, leasedLeaf+retiredSandboxTombstoneSuffix)
	assertMigrationPathMissing(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))

	failRemove = false
	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationPathMissing(t, scratchBase)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

type historyBackfillFake struct {
	completed map[string]bool
}

func (f *historyBackfillFake) ListRuntimeHistoryBackfills(_ context.Context, after protocol.Execution, _ int) ([]protocol.Execution, error) {
	var result []protocol.Execution
	for _, id := range []string{"a", "b"} {
		if !f.completed[id] && id > after.ID {
			result = append(result, protocol.Execution{ID: id, OwnerUserID: "owner", SessionKey: "session"})
		}
	}
	return result, nil
}
func (f *historyBackfillFake) RuntimeGraphForMigration(context.Context, protocol.Execution) (protocol.ExecutionRuntimeGraph, error) {
	return protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{ID: "launch", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Agent", SubjectID: "launch-use"}}}, nil
}
func (f *historyBackfillFake) CompleteRuntimeHistoryBackfill(_ context.Context, e protocol.Execution, _ protocol.ExecutionRuntimeGraph) error {
	f.completed[e.ID] = true
	return nil
}

func TestWorkGraphHistoryRetriesFailuresAndReadsEachSessionOnce(t *testing.T) {
	repository := &historyBackfillFake{completed: map[string]bool{}}
	calls := 0
	unavailable := errors.New("transcript unavailable")
	source := func(ctx context.Context, owner, session string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
		calls++
		if authctx.OwnerUserID(ctx) != owner || session != "session" {
			t.Fatal("migration lost owner scope")
		}
		return nil, nil, unavailable
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); !errors.Is(err, unavailable) {
		t.Fatalf("lost retryable failure: %v", err)
	}
	if calls != 1 || len(repository.completed) != 0 {
		t.Fatal("failed history was marked done or session was repeatedly read")
	}
	unavailable = nil
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(repository.completed) != 2 {
		t.Fatal("retry did not finish the frozen backlog")
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("completed history was read again")
	}
}
