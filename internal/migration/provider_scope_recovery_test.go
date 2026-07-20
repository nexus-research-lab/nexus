package migration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

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
	insertRecoveryAgent(t, db, "agent-new", "owner-a", "intentional-public")
	insertRecoveryProvider(t, db, "legacy-provider-id", "legacy-provider", "2026-05-25 00:00:00")
	insertRecoveryProvider(t, db, "intentional-public-id", "intentional-public", "2026-05-29 00:00:00")
	if _, err = db.Exec(`
UPDATE provider
SET created_at = datetime((
    SELECT tstamp
    FROM goose_db_version
    WHERE version_id = 18 AND is_applied = 1
    ORDER BY id DESC
    LIMIT 1
), '+1 second')
WHERE id = 'intentional-public-id'`); err != nil {
		t.Fatalf("设置迁移后公共 Provider 时间失败: %v", err)
	}
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

	var intentionalVisibility string
	if err = db.QueryRow(`SELECT visibility FROM provider WHERE id = 'intentional-public-id'`).Scan(&intentionalVisibility); err != nil {
		t.Fatalf("读取迁移后有意公共 Provider 失败: %v", err)
	}
	if intentionalVisibility != "public" {
		t.Fatalf("迁移后的有意公共 Provider visibility = %q, want public", intentionalVisibility)
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
	preferencesPath := filepath.Join(workspaceRoot, "owner-a", ".settings", "preferences.json")
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
