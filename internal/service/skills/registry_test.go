package skills

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServiceMigratesLegacyExternalSkillsToUsersThatUseThem(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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
	if _, err = os.Stat(filepath.Join(appfs.UserSkillDiscoveryRoot("owner-b"), "demo-skill", "SKILL.md")); err != nil {
		t.Fatalf("demo-skill 应迁移到 owner-b 用户级 Skill 源: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.UserSkillDiscoveryRoot("owner-a"), "shared-skill", "SKILL.md")); err != nil {
		t.Fatalf("shared-skill 应迁移到 owner-a 用户级 Skill 源: %v", err)
	}
	if _, err = os.Stat(filepath.Join(legacyRoot, "legacy-unassigned", "unused-skill", "SKILL.md")); err != nil {
		t.Fatalf("unused-skill 应归档到 legacy-unassigned: %v", err)
	}
	reloadedA, err := agentService.GetAgent(ctxA, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取迁移后的 owner-a agent 失败: %v", err)
	}
	reloadedB, err := agentService.GetAgent(ctxB, agentB.AgentID)
	if err != nil {
		t.Fatalf("读取迁移后的 owner-b agent 失败: %v", err)
	}
	if !slices.Contains(reloadedA.Options.SkillIDs, protocol.BuildExternalSkillReference("shared-skill")) {
		t.Fatalf("owner-a 未保存迁移后的外部 Skill 引用: %#v", reloadedA.Options.SkillIDs)
	}
	for _, skillName := range []string{"demo-skill", "shared-skill"} {
		if !slices.Contains(reloadedB.Options.SkillIDs, protocol.BuildExternalSkillReference(skillName)) {
			t.Fatalf("owner-b 未保存 %s 的迁移引用: %#v", skillName, reloadedB.Options.SkillIDs)
		}
		if _, statErr := os.Stat(filepath.Join(agentB.WorkspacePath, ".agents", "skills", skillName)); !os.IsNotExist(statErr) {
			t.Fatalf("owner-b 旧 workspace 副本 %s 未清理: %v", skillName, statErr)
		}
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

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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

func TestServiceResumesLegacyMigrationFromArchivedSource(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctx := ownerTestContext("owner-a")
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Owner A Agent"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "archived-skill"), 0o755); err != nil {
		t.Fatalf("创建旧 workspace 标记失败: %v", err)
	}

	legacyRoot := filepath.Join(cfg.CacheFileDir, "skills", "registry")
	archivedRoot := filepath.Join(legacyRoot, registryLegacyMigratedDirName, "archived-skill")
	writeTestSkillDir(t, archivedRoot, "archived-skill", "Archived Skill", true)

	if _, err = service.ListSkills(ctx, Query{}); err != nil {
		t.Fatalf("从归档源恢复 Skill 失败: %v", err)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取恢复后的 agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, protocol.BuildExternalSkillReference("archived-skill")) {
		t.Fatalf("归档迁移未恢复外部引用: %#v", reloaded.Options.SkillIDs)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "archived-skill")); !os.IsNotExist(err) {
		t.Fatalf("归档迁移后旧 workspace 标记未清理: %v", err)
	}
	if _, err = os.Stat(filepath.Join(legacyRoot, legacyRegistryMigrationMarkerName)); err != nil {
		t.Fatalf("归档迁移未写完成标记: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.UserSkillDiscoveryRoot("owner-a"), "archived-skill", "SKILL.md")); err != nil {
		t.Fatalf("归档全局源未恢复到 owner 用户级源: %v", err)
	}
}

func TestServiceMigratesLegacyOwnerRegistryIntoUserLibrary(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ownerUserID := "owner/a"
	ownerSegment := legacyOwnerSegment(ownerUserID)
	ctx := ownerTestContext(ownerUserID)
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Owner A Agent"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	markerPath := filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "owner-skill")
	if err = os.MkdirAll(markerPath, 0o755); err != nil {
		t.Fatalf("创建旧 workspace 标记失败: %v", err)
	}

	legacyRoot := filepath.Join(cfg.CacheFileDir, "skills", "registry", registryUsersDirName, ownerSegment)
	writeTestSkillDir(t, filepath.Join(legacyRoot, "owner-skill"), "owner-skill", "Owner Skill", true)
	if _, err = service.ListSkills(ctx, Query{}); err != nil {
		t.Fatalf("迁移旧 owner registry 失败: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.UserSkillDiscoveryRoot(ownerUserID), "owner-skill", "SKILL.md")); err != nil {
		t.Fatalf("旧 owner registry 未迁移到用户级源: %v", err)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取迁移后的 agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, protocol.BuildExternalSkillReference("owner-skill")) {
		t.Fatalf("旧 owner registry 未恢复 Agent 引用: %#v", reloaded.Options.SkillIDs)
	}
	if _, err = os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("旧 owner workspace 标记未清理: %v", err)
	}
	if _, err = os.Stat(filepath.Join(cfg.CacheFileDir, "skills", "registry", registryLegacyMigratedDirName, registryUsersDirName, ownerSegment)); err != nil {
		t.Fatalf("旧 owner registry 未归档: %v", err)
	}
}
