package skills

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

func TestServiceMigratesLegacyExternalSkillsToUsersThatUseThem(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctxA := ownerTestContext("owner-a")
	ctxB := ownerTestContext("owner-b")

	agentA, err := agentService.CreateAgent(ctxA, protocol.CreateRequest{Name: "Owner A Agent"})
	if err != nil {
		t.Fatalf("创建 owner-a agent 失败: %v", err)
	}
	agentB, err := agentService.CreateAgent(ctxB, protocol.CreateRequest{Name: "Owner B Agent"})
	if err != nil {
		t.Fatalf("创建 owner-b agent 失败: %v", err)
	}

	legacyRoot := filepath.Join(cfg.CacheFileDir, "skills", "registry")
	writeTestSkillDir(t, filepath.Join(legacyRoot, "demo-skill"), "demo-skill", "Demo Skill", true)
	writeTestSkillDir(t, filepath.Join(legacyRoot, "shared-skill"), "shared-skill", "Shared Skill", true)
	writeTestSkillDir(t, filepath.Join(legacyRoot, "unused-skill"), "unused-skill", "Unused Skill", true)
	if err = os.MkdirAll(filepath.Join(agentB.WorkspacePath, ".agents", "skills", "demo-skill"), 0o755); err != nil {
		t.Fatalf("标记 owner-b 使用 demo-skill 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(agentA.WorkspacePath, ".agents", "skills", "shared-skill"), 0o755); err != nil {
		t.Fatalf("标记 owner-a 使用 shared-skill 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(agentB.WorkspacePath, ".agents", "skills", "shared-skill"), 0o755); err != nil {
		t.Fatalf("标记 owner-b 使用 shared-skill 失败: %v", err)
	}

	itemsA, err := service.ListSkills(ctxA, Query{})
	if err != nil {
		t.Fatalf("迁移后读取 owner-a skills 失败: %v", err)
	}
	itemsB, err := service.ListSkills(ctxB, Query{})
	if err != nil {
		t.Fatalf("迁移后读取 owner-b skills 失败: %v", err)
	}
	if _, ok := findSkill(itemsA, "demo-skill"); ok {
		t.Fatalf("owner-a 不应看到只被 owner-b 使用的 demo-skill: %+v", itemsA)
	}
	if _, ok := findSkill(itemsB, "demo-skill"); !ok {
		t.Fatalf("owner-b 应看到 demo-skill: %+v", itemsB)
	}
	if _, ok := findSkill(itemsA, "shared-skill"); !ok {
		t.Fatalf("owner-a 应看到 shared-skill: %+v", itemsA)
	}
	if _, ok := findSkill(itemsB, "shared-skill"); !ok {
		t.Fatalf("owner-b 应看到 shared-skill: %+v", itemsB)
	}
	if _, ok := findSkill(itemsA, "unused-skill"); ok {
		t.Fatalf("owner-a 不应看到未使用 legacy skill: %+v", itemsA)
	}
	if _, ok := findSkill(itemsB, "unused-skill"); ok {
		t.Fatalf("owner-b 不应看到未使用 legacy skill: %+v", itemsB)
	}
	if _, err = os.Stat(filepath.Join(legacyRoot, "users", "owner-b", "demo-skill", "SKILL.md")); err != nil {
		t.Fatalf("demo-skill 应迁移到 owner-b 私有 registry: %v", err)
	}
	if _, err = os.Stat(filepath.Join(legacyRoot, "users", "owner-a", "shared-skill", "SKILL.md")); err != nil {
		t.Fatalf("shared-skill 应迁移到 owner-a 私有 registry: %v", err)
	}
	if _, err = os.Stat(filepath.Join(legacyRoot, "legacy-unassigned", "unused-skill", "SKILL.md")); err != nil {
		t.Fatalf("unused-skill 应归档到 legacy-unassigned: %v", err)
	}
}

func TestServiceExternalSkillRegistryIsPrivatePerOwner(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctxA := ownerTestContext("owner-a")
	ctxB := ownerTestContext("owner-b")

	sourceA := filepath.Join(t.TempDir(), "private-skill-a")
	sourceB := filepath.Join(t.TempDir(), "private-skill-b")
	writeTestSkillDir(t, sourceA, "private-skill", "Owner A Skill", false)
	writeTestSkillDir(t, sourceB, "private-skill", "Owner B Skill", false)
	if _, err = service.ImportLocalPath(ctxA, sourceA); err != nil {
		t.Fatalf("owner-a 导入 skill 失败: %v", err)
	}
	if _, err = service.ImportLocalPath(ctxB, sourceB); err != nil {
		t.Fatalf("owner-b 导入 skill 失败: %v", err)
	}

	itemsA, err := service.ListSkills(ctxA, Query{SourceType: sourceTypeExternal})
	if err != nil {
		t.Fatalf("读取 owner-a external skills 失败: %v", err)
	}
	itemsB, err := service.ListSkills(ctxB, Query{SourceType: sourceTypeExternal})
	if err != nil {
		t.Fatalf("读取 owner-b external skills 失败: %v", err)
	}
	skillA, ok := findSkill(itemsA, "private-skill")
	if !ok || skillA.Title != "Owner A Skill" {
		t.Fatalf("owner-a 应看到自己的 skill 版本: %+v", itemsA)
	}
	skillB, ok := findSkill(itemsB, "private-skill")
	if !ok || skillB.Title != "Owner B Skill" {
		t.Fatalf("owner-b 应看到自己的 skill 版本: %+v", itemsB)
	}

	if err = service.DeleteSkill(ctxA, "private-skill"); err != nil {
		t.Fatalf("owner-a 删除 skill 失败: %v", err)
	}
	itemsA, err = service.ListSkills(ctxA, Query{SourceType: sourceTypeExternal})
	if err != nil {
		t.Fatalf("删除后读取 owner-a external skills 失败: %v", err)
	}
	itemsB, err = service.ListSkills(ctxB, Query{SourceType: sourceTypeExternal})
	if err != nil {
		t.Fatalf("删除后读取 owner-b external skills 失败: %v", err)
	}
	if _, ok = findSkill(itemsA, "private-skill"); ok {
		t.Fatalf("owner-a 删除后不应继续看到 private-skill: %+v", itemsA)
	}
	if skillB, ok = findSkill(itemsB, "private-skill"); !ok || skillB.Title != "Owner B Skill" {
		t.Fatalf("owner-a 删除不应影响 owner-b: %+v", itemsB)
	}
}
