// INPUT: v0.1.27 registry、Agent workspace 副本与 runtime Skill 选择。
// OUTPUT: 验证 owner 共享源、稳定引用、完成标记和版本边界。
// POS: 上一发布版本 Skill 数据迁移的端到端回归测试。
package migration

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/pressly/goose/v3"
)

func TestRunLegacySkillStorageMigratesV027DataOnly(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".nexus")
	workspaceRoot := filepath.Join(configRoot, "workspace")
	cacheRoot := filepath.Join(configRoot, "cache")
	databaseURL := filepath.Join(configRoot, "data", "nexus.db")
	ownerAID := "owner/a"
	ownerADir := "owner_a"
	if err := os.MkdirAll(filepath.Dir(databaseURL), 0o755); err != nil {
		t.Fatalf("创建测试数据库目录失败: %v", err)
	}

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, legacySkillMigrationDir(t)); err != nil {
		t.Fatalf("执行基础 migration 失败: %v", err)
	}

	ownerAWorkspace := filepath.Join(workspaceRoot, ownerADir, "agent-a")
	ownerBWorkspace := filepath.Join(workspaceRoot, "owner-b", "agent-b")
	insertLegacySkillMigrationAgent(t, db, "agent-a", ownerAID, ownerAWorkspace, []string{"imagegen", "owner-only"})
	insertLegacySkillMigrationAgent(t, db, "agent-b", "owner-b", ownerBWorkspace, []string{"imagegen"})

	ownerSource := filepath.Join(cacheRoot, "skills", "registry", "users", ownerADir, "owner-only")
	reservedNameSource := filepath.Join(cacheRoot, "skills", "registry", "users", ownerADir, "users")
	shadowSource := filepath.Join(cacheRoot, "skills", "registry", "users", ownerADir, "local-shadow")
	globalSource := filepath.Join(cacheRoot, "skills", "registry", "shared-skill")
	archivedSource := filepath.Join(cacheRoot, "skills", "registry", "legacy-migrated", "archived-skill")
	writeLegacySkillMigrationSource(t, ownerSource, "owner-only", "owner-only\n")
	writeLegacySkillMigrationSource(t, reservedNameSource, "users", "reserved-name\n")
	writeLegacySkillMigrationSource(t, shadowSource, "local-shadow", "owner-source\n")
	writeLegacySkillMigrationSource(t, globalSource, "shared-skill", "shared-skill\n")
	writeLegacySkillMigrationSource(t, archivedSource, "archived-skill", "archived-skill\n")
	writeLegacySkillMigrationSource(
		t,
		filepath.Join(ownerAWorkspace, ".agents", "skills", "owner-only"),
		"owner-only",
		"owner-only\n",
	)
	writeLegacySkillMigrationSource(
		t,
		filepath.Join(ownerAWorkspace, ".agents", "skills", "shared-skill"),
		"shared-skill",
		"shared-skill\n",
	)
	writeMigrationTestFile(
		t,
		filepath.Join(ownerAWorkspace, ".agents", "skills", "local-shadow", "SKILL.md"),
		"workspace-local\n",
	)
	writeLegacySkillMigrationSource(
		t,
		filepath.Join(ownerBWorkspace, ".agents", "skills", "shared-skill"),
		"shared-skill",
		"shared-skill\n",
	)

	intermediateSource := filepath.Join(
		configRoot,
		"users",
		ownerADir,
		".agents",
		"skills",
		"intermediate-skill",
	)
	writeLegacySkillMigrationSource(t, intermediateSource, "intermediate-skill", "intermediate\n")
	unpublishedWorkspaceSource := filepath.Join(ownerAWorkspace, ".agents", "unpublished-skill")
	writeLegacySkillMigrationSource(t, unpublishedWorkspaceSource, "unpublished-skill", "unpublished\n")

	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    databaseURL,
		WorkspacePath:  workspaceRoot,
		CacheFileDir:   cacheRoot,
	}
	if err = RunLegacySkillStorage(t.Context(), cfg, configRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行 v0.1.27 Skill 存储迁移失败: %v", err)
	}

	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "owner-only", "SKILL.md"),
		"owner-only\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "users", "SKILL.md"),
		"reserved-name\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, "owner-b", ".agents", "skills", "shared-skill", "SKILL.md"),
		"shared-skill\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "shared-skill", "SKILL.md"),
		"shared-skill\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, ".agents", "skills", "archived-skill", "SKILL.md"),
		"archived-skill\n",
	)
	assertMigrationPathMissing(t, filepath.Join(ownerAWorkspace, ".agents", "skills", "owner-only"))
	assertMigrationPathMissing(t, filepath.Join(ownerAWorkspace, ".agents", "skills", "shared-skill"))
	assertMigrationPathMissing(t, filepath.Join(ownerBWorkspace, ".agents", "skills", "shared-skill"))
	assertMigrationFileContent(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "local-shadow", "SKILL.md"),
		"owner-source\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(ownerAWorkspace, ".agents", "skills", "local-shadow", "SKILL.md"),
		"workspace-local\n",
	)

	ownerAReferences := readLegacySkillMigrationReferences(t, db, "agent-a")
	if !slices.Equal(ownerAReferences, []string{"imagegen", "external:owner-only", "external:shared-skill"}) {
		t.Fatalf("owner-a Skill 引用 = %#v", ownerAReferences)
	}
	ownerBReferences := readLegacySkillMigrationReferences(t, db, "agent-b")
	if !slices.Equal(ownerBReferences, []string{"imagegen", "external:shared-skill"}) {
		t.Fatalf("owner-b Skill 引用 = %#v", ownerBReferences)
	}
	assertCompletedMigrationMarker(t, configRoot, legacySkillStorageMigrationName)

	// 未发布的中间目录不属于升级兼容面。
	assertMigrationPathExists(t, intermediateSource)
	assertMigrationPathMissing(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "intermediate-skill"),
	)
	assertMigrationPathExists(t, unpublishedWorkspaceSource)

	lateSource := filepath.Join(cacheRoot, "skills", "registry", "users", ownerADir, "late-skill")
	writeLegacySkillMigrationSource(t, lateSource, "late-skill", "late\n")
	if err = RunLegacySkillStorage(t.Context(), cfg, configRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行 v0.1.27 Skill 存储迁移失败: %v", err)
	}
	assertMigrationPathExists(t, lateSource)
	assertMigrationPathMissing(
		t,
		filepath.Join(workspaceRoot, ownerADir, ".agents", "skills", "late-skill"),
	)
}

func insertLegacySkillMigrationAgent(
	t *testing.T,
	db *sql.DB,
	agentID string,
	ownerUserID string,
	workspacePath string,
	skillIDs []string,
) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO users (user_id, username, display_name, role, status)
VALUES (?, ?, ?, 'member', 'active')`,
		ownerUserID,
		ownerUserID,
		ownerUserID,
	); err != nil {
		t.Fatalf("插入测试用户失败: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', 'active', ?, ?, 0)`,
		agentID,
		agentID,
		agentID,
		workspacePath,
		ownerUserID,
	); err != nil {
		t.Fatalf("插入测试 Agent 失败: %v", err)
	}
	payload, err := json.Marshal(skillIDs)
	if err != nil {
		t.Fatalf("编码测试 Skill 引用失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO runtimes (
    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
    mcp_servers_json, skill_ids_json, setting_sources_json, runtime_version
) VALUES (?, ?, '', '', '[]', '[]', '{}', ?, '[]', 1)`,
		"runtime-"+agentID,
		agentID,
		string(payload),
	); err != nil {
		t.Fatalf("插入测试 runtime 失败: %v", err)
	}
}

func writeLegacySkillMigrationSource(t *testing.T, root string, name string, content string) {
	t.Helper()
	manifest, err := json.Marshal(legacySkillManifest{Name: name, SourceType: "external"})
	if err != nil {
		t.Fatalf("编码测试 Skill manifest 失败: %v", err)
	}
	writeMigrationTestFile(t, filepath.Join(root, "SKILL.md"), content)
	writeMigrationTestFile(t, filepath.Join(root, ".nexus-skill.json"), string(manifest))
}

func readLegacySkillMigrationReferences(t *testing.T, db *sql.DB, agentID string) []string {
	t.Helper()
	var payload string
	if err := db.QueryRow(`SELECT skill_ids_json FROM runtimes WHERE agent_id = ?`, agentID).Scan(&payload); err != nil {
		t.Fatalf("读取迁移后的 Skill 引用失败: %v", err)
	}
	var result []string
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("解析迁移后的 Skill 引用失败: %v", err)
	}
	return result
}

func assertMigrationFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取迁移文件失败 %q: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("迁移文件内容 %q = %q, want %q", path, content, expected)
	}
}

func legacySkillMigrationDir(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
