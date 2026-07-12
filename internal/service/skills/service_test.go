package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"

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
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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
	if !containsSkill(items, "imagegen") {
		t.Fatalf("图片生成系统 skill 未暴露: %+v", items)
	}
	if containsSkill(items, "scheduled-task-manager") {
		t.Fatalf("定时任务已由 nexus_automation 工具承载，不应再暴露重复 skill: %+v", items)
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

	legacyLocalSkillRoot := filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "claude-agent-skill")
	if err = os.MkdirAll(legacyLocalSkillRoot, 0o755); err != nil {
		t.Fatalf("创建 agent legacy 本地 skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(legacyLocalSkillRoot, "SKILL.md"), []byte(`---
name: claude-agent-skill
title: Claude Agent Skill
description: 兼容旧版 runtime 在 .claude/skills 下创建的技能目录
---

# claude-agent-skill
`), 0o644); err != nil {
		t.Fatalf("写入 agent legacy 本地 skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取含 agent legacy 本地 skill 的列表失败: %v", err)
	}
	legacyAgentSkill, ok := findSkill(items, "claude-agent-skill")
	if !ok {
		t.Fatalf("agent legacy 本地 skill 未暴露: %+v", items)
	}
	if legacyAgentSkill.SourceType != sourceTypeWorkspace || !legacyAgentSkill.Installed || legacyAgentSkill.Locked {
		t.Fatalf("agent legacy 本地 skill 状态不正确: %+v", legacyAgentSkill)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "claude-agent-skill"); err != nil {
		t.Fatalf("agent legacy 本地 skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(legacyLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("agent legacy 本地 skill 移除后目录仍存在: %v", err)
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

func TestUpdateSingleSkillReportsRedeployFailureAndContinues(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewServiceWithDB(cfg, db, agentService, workspaceService)
	ctx := context.Background()

	repoV1 := filepath.Join(t.TempDir(), "repo-v1")
	repoV2 := filepath.Join(t.TempDir(), "repo-v2")
	writeTestSkillDir(t, filepath.Join(repoV1, "skills", "git-skill"), "git-skill", "Git Skill v1", false)
	writeTestSkillDir(t, filepath.Join(repoV2, "skills", "git-skill"), "git-skill", "Git Skill v2", false)
	activeRepo := repoV1
	activeCommit := "commit-v1"
	service.commandRunner = func(_ context.Context, workDir string, _ []string, command ...string) (string, error) {
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "clone") {
			return "", copyDirectory(activeRepo, command[len(command)-1])
		}
		if len(command) >= 3 && command[0] == "git" && command[1] == "rev-parse" && workDir != "" {
			return activeCommit, nil
		}
		return "", nil
	}

	failingAgent, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "失败助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	successAgent, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "成功助手"})
	if err != nil {
		t.Fatalf("创建第二个 agent 失败: %v", err)
	}
	if _, err = service.ImportGitPath(ctx, "https://example.com/skills.git", "main", "skills/git-skill"); err != nil {
		t.Fatalf("Git 导入失败: %v", err)
	}
	for _, agentValue := range []protocol.Agent{*failingAgent, *successAgent} {
		if _, err = service.InstallSkill(ctx, agentValue.AgentID, "git-skill"); err != nil {
			t.Fatalf("安装 Git skill 到 %s 失败: %v", agentValue.AgentID, err)
		}
	}
	successSkillPath := filepath.Join(successAgent.WorkspacePath, ".agents", "skills", "git-skill", "SKILL.md")
	payload, err := os.ReadFile(successSkillPath)
	if err != nil {
		t.Fatalf("读取已安装 skill 失败: %v", err)
	}
	if !strings.Contains(string(payload), "Git Skill v1") {
		t.Fatalf("初始安装内容不正确: %s", payload)
	}

	makeSkillDeploymentRootReadOnly(t, failingAgent.WorkspacePath)
	activeRepo = repoV2
	activeCommit = "commit-v2"
	detail, err := service.UpdateSingleSkill(ctx, "git-skill")
	if err != nil {
		t.Fatalf("更新 Git skill 失败: %v", err)
	}
	if len(detail.DeployFailures) != 1 || detail.DeployFailures[0].AgentID != failingAgent.AgentID {
		t.Fatalf("未返回失败 Agent 信息: %+v", detail.DeployFailures)
	}
	if len(detail.DeploySuccesses) != 1 || detail.DeploySuccesses[0].AgentID != successAgent.AgentID {
		t.Fatalf("未返回成功 Agent 信息: %+v", detail.DeploySuccesses)
	}
	payload, err = os.ReadFile(successSkillPath)
	if err != nil {
		t.Fatalf("读取更新后 skill 失败: %v", err)
	}
	if !strings.Contains(string(payload), "Git Skill v2") {
		t.Fatalf("成功 Agent 的 skill 未随库更新: %s", payload)
	}
}

func containsSkill(items []Info, target string) bool {
	return slices.ContainsFunc(items, func(item Info) bool {
		return item.Name == target
	})
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

func makeSkillDeploymentRootReadOnly(t *testing.T, workspacePath string) {
	t.Helper()
	skillRoot := filepath.Join(workspacePath, ".agents", "skills")
	if err := os.Chmod(skillRoot, 0o555); err != nil {
		t.Fatalf("设置只读 skill 目录失败: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(skillRoot, 0o755)
	})
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
