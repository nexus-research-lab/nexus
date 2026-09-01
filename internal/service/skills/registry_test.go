package skills

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServiceExternalSkillRegistryIsPrivatePerOwner(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db := openSkillsTestDB(t, cfg)

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctxA := ownerTestContext("owner-a")
	ctxB := ownerTestContext("owner-b")

	sourceA := filepath.Join(t.TempDir(), "private-skill-a")
	sourceB := filepath.Join(t.TempDir(), "private-skill-b")
	writeTestSkillDir(t, sourceA, "private-skill", "Owner A Skill", false)
	writeTestSkillDir(t, sourceB, "private-skill", "Owner B Skill", false)
	if _, err := service.ImportLocalPath(ctxA, sourceA); err != nil {
		t.Fatalf("owner-a 导入 skill 失败: %v", err)
	}
	if _, err := service.ImportLocalPath(ctxB, sourceB); err != nil {
		t.Fatalf("owner-b 导入 skill 失败: %v", err)
	}
	agentA, err := agentService.CreateAgent(ctxA, protocol.CreateRequest{Name: "Owner A Agent"})
	if err != nil {
		t.Fatalf("创建 owner-a Agent 失败: %v", err)
	}
	if _, err = agentService.UpdateAgentSkillSelection(
		ctxA,
		agentA.AgentID,
		[]string{"private-skill"},
		[]string{"private-skill"},
	); err != nil {
		t.Fatalf("准备 skill 引用失败: %v", err)
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
	reloadedAgent, err := agentService.GetAgent(ctxA, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取删除 skill 后的 Agent 失败: %v", err)
	}
	if slices.Contains(reloadedAgent.Options.SkillIDs, "private-skill") ||
		slices.Contains(reloadedAgent.Options.DisabledSkillIDs, "private-skill") {
		t.Fatalf("删除 skill 后仍残留 Agent 引用: %+v", reloadedAgent.Options)
	}
}
