package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestServiceImportsAndInstallsSkill(t *testing.T) {
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
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "技能测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	items, err := service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 agent 技能失败: %v", err)
	}
	if !containsSkill(items, "memory-manager") {
		t.Fatalf("系统托管 skill 未暴露: %+v", items)
	}
	if !containsSkill(items, "imagegen") {
		t.Fatalf("图片生成系统 skill 未暴露: %+v", items)
	}
	if !containsSkill(items, "scheduled-task-manager") {
		t.Fatalf("定时任务系统 skill 未暴露: %+v", items)
	}
	if !containsSkill(items, "goal-manager") {
		t.Fatalf("Goal 系统 skill 未暴露: %+v", items)
	}
	if containsSkill(items, "room-playbook") {
		t.Fatalf("room scope skill 不应暴露为 agent 技能: %+v", items)
	}
	roomSkills, err := service.ListSkills(ctx, Query{Scope: ScopeRoom})
	if err != nil {
		t.Fatalf("读取 room skill 列表失败: %v", err)
	}
	roomSkill, ok := findSkill(roomSkills, "room-playbook")
	if !ok {
		t.Fatalf("未读取到内置 room skill: %+v", roomSkills)
	}
	if roomSkill.Scope != ScopeRoom {
		t.Fatalf("room skill scope 不正确: %+v", roomSkill)
	}
	werewolfSkill, ok := findSkill(roomSkills, "werewolf-6p")
	if !ok {
		t.Fatalf("未读取到狼人杀 room skill: %+v", roomSkills)
	}
	if werewolfSkill.Scope != ScopeRoom {
		t.Fatalf("狼人杀 room skill scope 不正确: %+v", werewolfSkill)
	}
	if _, err = service.GetSkillDetail(ctx, "room-playbook", agentValue.AgentID); err == nil {
		t.Fatal("room scope skill 不应作为 agent skill 详情读取")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "room-playbook"); err == nil {
		t.Fatal("room scope skill 不应允许安装到 agent")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "scheduled-task-manager"); err == nil {
		t.Fatal("系统托管 scheduled-task-manager skill 不应允许手动安装")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "goal-manager"); err == nil {
		t.Fatal("系统托管 goal-manager skill 不应允许手动安装")
	}

	agentLocalSkillRoot := filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "agent-only-skill")
	if err = os.MkdirAll(agentLocalSkillRoot, 0o755); err != nil {
		t.Fatalf("创建 agent 本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(agentLocalSkillRoot, "SKILL.md"), []byte(`---
name: agent-only-skill
title: Agent Only Skill
description: 只在当前智能体工作区内可用
tags: [agent-local]
---

# agent-only-skill

workspace skill body
`), 0o644); err != nil {
		t.Fatalf("写入 agent 本地 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取含 agent 本地 skill 的列表失败: %v", err)
	}
	agentLocalSkill, ok := findSkill(items, "agent-only-skill")
	if !ok {
		t.Fatalf("agent 本地 skill 未暴露: %+v", items)
	}
	if agentLocalSkill.SourceType != sourceTypeWorkspace || !agentLocalSkill.Installed || agentLocalSkill.Locked {
		t.Fatalf("agent 本地 skill 状态不正确: %+v", agentLocalSkill)
	}
	if _, err = service.GetSkillDetail(ctx, "agent-only-skill", ""); err == nil {
		t.Fatal("未指定 agent 时不应读取 agent 本地 skill")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "agent-only-skill"); err == nil {
		t.Fatal("agent 本地 skill 不应允许通过市场安装")
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "agent-only-skill"); err != nil {
		t.Fatalf("agent 本地 skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(agentLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("agent 本地 skill 移除后目录仍存在: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("移除 agent 本地 skill 后读取列表失败: %v", err)
	}
	if _, ok := findSkill(items, "agent-only-skill"); ok {
		t.Fatalf("agent 本地 skill 移除后仍在列表中: %+v", items)
	}

	directAgentLocalSkillRoot := filepath.Join(agentValue.WorkspacePath, ".agents", "direct-agent-skill")
	if err = os.MkdirAll(directAgentLocalSkillRoot, 0o755); err != nil {
		t.Fatalf("创建 agent 直属本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(directAgentLocalSkillRoot, "SKILL.md"), []byte(`---
name: direct-agent-skill
title: Direct Agent Skill
description: 兼容直接位于 .agents 下的技能目录
---

# direct-agent-skill
`), 0o644); err != nil {
		t.Fatalf("写入 agent 直属本地 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取含 agent 直属本地 skill 的列表失败: %v", err)
	}
	directAgentLocalSkill, ok := findSkill(items, "direct-agent-skill")
	if !ok {
		t.Fatalf("agent 直属本地 skill 未暴露: %+v", items)
	}
	if directAgentLocalSkill.SourceType != sourceTypeWorkspace || !directAgentLocalSkill.Installed || directAgentLocalSkill.Locked {
		t.Fatalf("agent 直属本地 skill 状态不正确: %+v", directAgentLocalSkill)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "direct-agent-skill"); err != nil {
		t.Fatalf("agent 直属本地 skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(directAgentLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("agent 直属本地 skill 移除后目录仍存在: %v", err)
	}

	claudeLocalSkillRoot := filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "claude-agent-skill")
	if err = os.MkdirAll(claudeLocalSkillRoot, 0o755); err != nil {
		t.Fatalf("创建 agent Claude 本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(claudeLocalSkillRoot, "SKILL.md"), []byte(`---
name: claude-agent-skill
title: Claude Agent Skill
description: 兼容 Agent 在 .claude/skills 下创建的技能目录
---

# claude-agent-skill
`), 0o644); err != nil {
		t.Fatalf("写入 agent Claude 本地 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取含 agent Claude 本地 skill 的列表失败: %v", err)
	}
	claudeAgentSkill, ok := findSkill(items, "claude-agent-skill")
	if !ok {
		t.Fatalf("agent Claude 本地 skill 未暴露: %+v", items)
	}
	if claudeAgentSkill.SourceType != sourceTypeWorkspace || !claudeAgentSkill.Installed || claudeAgentSkill.Locked {
		t.Fatalf("agent Claude 本地 skill 状态不正确: %+v", claudeAgentSkill)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "claude-agent-skill"); err != nil {
		t.Fatalf("agent Claude 本地 skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(claudeLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("agent Claude 本地 skill 移除后目录仍存在: %v", err)
	}

	localSkillRoot := filepath.Join(t.TempDir(), "demo-skill")
	if err = os.MkdirAll(localSkillRoot, 0o755); err != nil {
		t.Fatalf("创建本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(localSkillRoot, "SKILL.md"), []byte(`---
name: demo-skill
title: Demo Skill
description: 这是一个测试技能
tags: [demo, test]
---

# demo-skill

skill body
`), 0o644); err != nil {
		t.Fatalf("写入本地 skill 失败: %v", err)
	}

	imported, err := service.ImportLocalPath(ctx, localSkillRoot)
	if err != nil {
		t.Fatalf("导入本地 skill 失败: %v", err)
	}
	if imported.Name != "demo-skill" {
		t.Fatalf("导入的 skill 名称不正确: %+v", imported)
	}

	installed, err := service.InstallSkill(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("安装 skill 失败: %v", err)
	}
	if !installed.Installed {
		t.Fatalf("安装后状态不正确: %+v", installed)
	}

	if err = service.UninstallSkill(ctx, agentValue.AgentID, "demo-skill"); err != nil {
		t.Fatalf("卸载 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("再次读取 agent 技能失败: %v", err)
	}
	for _, item := range items {
		if item.Name == "demo-skill" && item.Installed {
			t.Fatalf("卸载后仍显示 installed: %+v", item)
		}
	}
}

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

func TestSkillResponseSlicesMarshalAsEmptyArray(t *testing.T) {
	info := Info{
		Name: "demo-skill",
		Tags: firstNonEmptySlice(nil),
	}
	payload, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("序列化技能信息失败: %v", err)
	}
	if string(payload) == "" || !strings.Contains(string(payload), `"tags":[]`) {
		t.Fatalf("tags 未按协议序列化为空数组: %s", string(payload))
	}
}

func TestToStringSliceReturnsEmptySlice(t *testing.T) {
	if result := toStringSlice(""); result == nil || len(result) != 0 {
		t.Fatalf("空字符串应返回空切片，实际: %#v", result)
	}
	if result := toStringSlice(nil); result == nil || len(result) != 0 {
		t.Fatalf("nil 应返回空切片，实际: %#v", result)
	}
}

func TestParseSkillFrontmatterWithoutTagsReturnsEmptySlice(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: demo-skill
title: Demo Skill
description: no tags here
---

# Demo Skill
`, "demo-skill")
	if parsed.Tags == nil {
		t.Fatal("未声明 tags 时也必须返回空切片，不能是 nil")
	}
	if len(parsed.Tags) != 0 {
		t.Fatalf("未声明 tags 时应为空切片，实际: %#v", parsed.Tags)
	}
}

func TestParseSkillFrontmatterBlockDescription(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: chronicle
description: |
  Allows you to view the user's screen as well as several hours of history.

  Use this skill when recent screen context is needed.
tags: [screen, context]
---

# Chronicle
`, "chronicle")
	if parsed.Description == "" || strings.Contains(parsed.Description, "|") {
		t.Fatalf("多行 description 解析不正确: %q", parsed.Description)
	}
	if !strings.Contains(parsed.Description, "recent screen context") {
		t.Fatalf("多行 description 内容丢失: %q", parsed.Description)
	}
	if len(parsed.Tags) != 2 || parsed.Tags[0] != "screen" || parsed.Tags[1] != "context" {
		t.Fatalf("block scalar 后续字段解析不正确: %#v", parsed.Tags)
	}
}

func containsSkill(items []Info, target string) bool {
	for _, item := range items {
		if item.Name == target {
			return true
		}
	}
	return false
}

func findSkill(items []Info, target string) (Info, bool) {
	for _, item := range items {
		if item.Name == target {
			return item, true
		}
	}
	return Info{}, false
}

func ownerTestContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func writeTestSkillDir(t *testing.T, root string, name string, title string, withManifest bool) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建测试 skill 目录失败: %v", err)
	}
	content := `---
name: ` + name + `
title: ` + title + `
description: 测试技能
tags: [test]
---

# ` + name + `
`
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("写入测试 SKILL.md 失败: %v", err)
	}
	if !withManifest {
		return
	}
	manifest := externalManifest{
		Name:           name,
		Title:          title,
		Description:    "测试技能",
		Scope:          scopeAny,
		CategoryKey:    "custom-imports",
		CategoryName:   "自定义导入",
		Version:        "legacy",
		SourceType:     sourceTypeExternal,
		SourceRef:      root,
		ImportMode:     "local_path",
		Recommendation: "legacy test skill",
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("序列化测试 skill manifest 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(root, ".nexus-skill.json"), payload, 0o644); err != nil {
		t.Fatalf("写入测试 skill manifest 失败: %v", err)
	}
}

func newSkillsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	return config.Config{
		Host:                      "127.0.0.1",
		Port:                      18012,
		ProjectName:               "nexus-skills-test",
		APIPrefix:                 "/nexus/v1",
		WebSocketPath:             "/nexus/v1/chat/ws",
		DefaultAgentID:            "nexus",
		WorkspacePath:             filepath.Join(root, "workspace"),
		CacheFileDir:              filepath.Join(root, "cache"),
		DatabaseDriver:            "sqlite",
		DatabaseURL:               filepath.Join(root, "nexus.db"),
		ConnectorOAuthRedirectURI: "http://localhost:3000/capability/connectors",
	}
}

func migrateSkillsSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, skillsTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func skillsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
