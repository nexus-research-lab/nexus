package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithSelectedAppSkills(
		m,
		"diagram-design",
		"execution-orchestrator",
		"goal-manager",
		"ima-skill",
		"imagegen",
		"visualize",
		"automation",
		"kami",
		"nexus-manager",
		"nexus-configuration",
		"room-playbook",
		"wechat-article-search",
		"werewolf-6p",
	))
}

func TestValidateSkillNameRejectsReservedCanonicalNames(t *testing.T) {
	for _, name := range []string{"", " padded", "padded ", "external:demo", "EXTERNAL:demo"} {
		if err := validateSkillName(name); err == nil {
			t.Fatalf("Skill 保留 canonical name 应被拒绝: %q", name)
		}
	}
	for _, name := range []string{"demo-skill", "财务分析"} {
		if err := validateSkillName(name); err != nil {
			t.Fatalf("有效 Skill canonical name 被拒绝 %q: %v", name, err)
		}
	}
}

func TestServiceImportsAndEnablesSkill(t *testing.T) {
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
	automationSkill, ok := findSkill(items, "automation")
	if !ok {
		t.Fatalf("自动化系统 skill 未暴露: %+v", items)
	}
	if automationSkill.SourceType != sourceTypeSystem || !automationSkill.Locked || automationSkill.Deletable || !automationSkill.EnabledForAgent {
		t.Fatalf("automation 应作为已启用的系统内置 skill: %+v", automationSkill)
	}
	if !containsSkill(items, "goal-manager") {
		t.Fatalf("Goal 系统 skill 未暴露: %+v", items)
	}
	executionSkill, ok := findSkill(items, "execution-orchestrator")
	if !ok || executionSkill.SourceType != sourceTypeSystem || !executionSkill.Locked || executionSkill.Deletable || !executionSkill.EnabledForAgent {
		t.Fatalf("Execution 应作为已启用且不可覆盖的系统 Skill: %+v", executionSkill)
	}
	visualizeSkill, ok := findSkill(items, "visualize")
	if !ok {
		t.Fatalf("可视化系统 skill 未暴露: %+v", items)
	}
	if visualizeSkill.SourceType != sourceTypeSystem || !visualizeSkill.Locked || visualizeSkill.Deletable || !visualizeSkill.EnabledForAgent {
		t.Fatalf("visualize 应与 imagegen 一样作为已启用的系统内置 skill: %+v", visualizeSkill)
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
	werewolfDetail, err := service.GetSkillDetail(ctx, "werewolf-6p", "")
	if err != nil {
		t.Fatalf("读取狼人杀 skill 详情失败: %v", err)
	}
	for _, rule := range []string{
		"Never expose the private actor, collector, target, decision, or who the host is waiting for",
		"The final pending actor for that night gets",
		"Nothing about who attacked, who saved or poisoned, why nobody died",
		"Announce only the resolved death list",
		"between 60 and 120 Chinese characters",
	} {
		if !strings.Contains(werewolfDetail.ReadmeMarkdown, rule) {
			t.Fatalf("狼人杀 skill 缺少闭环约束 %q", rule)
		}
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
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "execution-orchestrator"); err == nil {
		t.Fatal("系统托管 execution-orchestrator skill 不应允许手动安装")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "visualize"); err == nil {
		t.Fatal("系统托管 visualize skill 不应允许手动安装")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "automation"); err == nil {
		t.Fatal("系统托管 automation skill 不应允许手动安装")
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
	if agentLocalSkill.SourceType != sourceTypeWorkspace || !agentLocalSkill.EnabledForAgent || agentLocalSkill.Locked {
		t.Fatalf("agent 本地 skill 状态不正确: %+v", agentLocalSkill)
	}
	if agentLocalSkill.OriginKind != originKindAgentCreated ||
		agentLocalSkill.StorageScope != storageScopeAgent {
		t.Fatalf("Agent 创建 Skill 的来源投影不正确: %+v", agentLocalSkill)
	}
	agentLocalState, err := service.GetAgentSkillState(ctx, agentValue.AgentID, "agent-only-skill")
	if err != nil {
		t.Fatalf("读取 agent 本地 Skill 状态失败: %v", err)
	}
	if !agentLocalState.Available || !agentLocalState.Installed ||
		agentLocalState.SourceType != sourceTypeWorkspace ||
		agentLocalState.RuntimeVersion != agentValue.RuntimeVersion {
		t.Fatalf("agent 本地 Skill 状态快照不正确: %+v", agentLocalState)
	}
	if _, err = service.GetSkillDetail(ctx, "agent-only-skill", ""); err == nil {
		t.Fatal("Agent 工作区 Skill 不应进入全局目录")
	}
	if _, err = service.InstallSkill(ctx, agentValue.AgentID, "agent-only-skill"); err == nil {
		t.Fatal("agent 本地 skill 不应允许通过市场安装")
	}
	toggled, err := service.SetAgentSkillEnabled(
		ctx,
		agentValue.AgentID,
		"agent-only-skill",
		false,
	)
	if err != nil {
		t.Fatalf("停用 Agent 本地 Skill 失败: %v", err)
	}
	if toggled.EnabledForAgent {
		t.Fatalf("停用后 Agent 本地 Skill 仍显示启用: %+v", toggled)
	}
	if _, err = os.Stat(agentLocalSkillRoot); err != nil {
		t.Fatalf("停用 Agent 本地 Skill 不应删除目录: %v", err)
	}
	reloadedAgent, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取停用后的 Agent 失败: %v", err)
	}
	if !slices.Contains(reloadedAgent.Options.DisabledSkillIDs, "agent-only-skill") {
		t.Fatalf("Agent 未保存显式停用状态: %#v", reloadedAgent.Options.DisabledSkillIDs)
	}
	toggled, err = service.SetAgentSkillEnabled(
		ctx,
		agentValue.AgentID,
		"agent-only-skill",
		true,
	)
	if err != nil {
		t.Fatalf("重新启用 Agent 本地 Skill 失败: %v", err)
	}
	if !toggled.EnabledForAgent {
		t.Fatalf("重新启用后 Agent 本地 Skill 状态不正确: %+v", toggled)
	}
	reloadedAgent, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取重新启用后的 Agent 失败: %v", err)
	}
	if slices.Contains(reloadedAgent.Options.DisabledSkillIDs, "agent-only-skill") {
		t.Fatalf("重新启用后仍残留停用状态: %#v", reloadedAgent.Options.DisabledSkillIDs)
	}
	if _, err = service.SetAgentSkillEnabled(
		ctx,
		agentValue.AgentID,
		"agent-only-skill",
		false,
	); err != nil {
		t.Fatalf("删除前停用 Agent 本地 Skill 失败: %v", err)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "agent-only-skill"); err != nil {
		t.Fatalf("agent 本地 skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(agentLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("agent 本地 skill 移除后目录仍存在: %v", err)
	}
	agentLocalState, err = service.GetAgentSkillState(ctx, agentValue.AgentID, "agent-only-skill")
	if err != nil {
		t.Fatalf("读取移除后的 agent 本地 Skill 状态失败: %v", err)
	}
	if agentLocalState.Available || agentLocalState.Installed {
		t.Fatalf("移除后的 agent 本地 Skill 应报告不存在: %+v", agentLocalState)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("移除 agent 本地 skill 后读取列表失败: %v", err)
	}
	if _, ok := findSkill(items, "agent-only-skill"); ok {
		t.Fatalf("agent 本地 skill 移除后仍在列表中: %+v", items)
	}
	reloadedAgent, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取删除本地 Skill 后的 Agent 失败: %v", err)
	}
	if slices.Contains(reloadedAgent.Options.DisabledSkillIDs, "agent-only-skill") {
		t.Fatalf("删除本地 Skill 后不应残留停用状态: %#v", reloadedAgent.Options.DisabledSkillIDs)
	}

	claudeLocalSkillRoot := filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "claude-agent-skill")
	if err = os.MkdirAll(claudeLocalSkillRoot, 0o755); err != nil {
		t.Fatalf("创建 Agent Claude 本地 Skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(claudeLocalSkillRoot, "SKILL.md"), []byte(`---
name: wrong-claude-frontmatter-name
title: Claude Agent Skill
description: Claude 在 .claude/skills 下创建的技能目录
---

# claude-agent-skill
`), 0o644); err != nil {
		t.Fatalf("写入 Agent Claude 本地 Skill 失败: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取含 Agent Claude 本地 Skill 的列表失败: %v", err)
	}
	claudeAgentSkill, ok := findSkill(items, "claude-agent-skill")
	if !ok {
		t.Fatalf("Agent Claude 本地 Skill 未暴露: %+v", items)
	}
	if claudeAgentSkill.SourceType != sourceTypeWorkspace || !claudeAgentSkill.EnabledForAgent || claudeAgentSkill.Locked {
		t.Fatalf("Agent Claude 本地 Skill 状态不正确: %+v", claudeAgentSkill)
	}
	if _, exists := findSkill(items, "wrong-claude-frontmatter-name"); exists {
		t.Fatalf("Agent 本地 Skill 不应用 frontmatter name 改写 canonical name: %+v", items)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "claude-agent-skill"); err != nil {
		t.Fatalf("Agent Claude 本地 Skill 应允许从当前智能体移除: %v", err)
	}
	if _, err = os.Stat(claudeLocalSkillRoot); !os.IsNotExist(err) {
		t.Fatalf("Agent Claude 本地 Skill 移除后目录仍存在: %v", err)
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
	externalState, err := service.GetAgentSkillState(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("读取未安装外部 Skill 状态失败: %v", err)
	}
	if !externalState.Available || externalState.Installed ||
		externalState.SourceType != sourceTypeExternal {
		t.Fatalf("未安装外部 Skill 状态不正确: %+v", externalState)
	}

	enabled, err := service.InstallSkill(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("通过兼容入口启用 skill 失败: %v", err)
	}
	if !enabled.EnabledForAgent {
		t.Fatalf("启用后状态不正确: %+v", enabled)
	}
	externalState, err = service.GetAgentSkillState(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("读取已安装外部 Skill 状态失败: %v", err)
	}
	if !externalState.Available || !externalState.Installed ||
		externalState.SourceType != sourceTypeExternal {
		t.Fatalf("已安装外部 Skill 状态不正确: %+v", externalState)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取安装后的 agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, protocol.BuildExternalSkillReference("demo-skill")) {
		t.Fatalf("外部 Skill 应保存用户级引用: %#v", reloaded.Options.SkillIDs)
	}
	if _, statErr := os.Stat(filepath.Join(reloaded.WorkspacePath, ".agents", "skills", "demo-skill")); !os.IsNotExist(statErr) {
		t.Fatalf("外部 Skill 不应复制到 workspace: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(workspacepkg.UserSkillDiscoveryRoot(cfg, authctx.SystemUserID), "demo-skill", "SKILL.md")); statErr != nil {
		t.Fatalf("外部 Skill 用户级源不存在: %v", statErr)
	}
	globalItems, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取全局 Skill 库失败: %v", err)
	}
	globalDemoSkill, ok := findSkill(globalItems, "demo-skill")
	if !ok || globalDemoSkill.EnabledAgentCount != 1 {
		t.Fatalf("全局 Skill 未正确统计 Agent 使用数: %+v", globalDemoSkill)
	}
	bindings, err := service.ListSkillAgents(ctx, "demo-skill")
	if err != nil {
		t.Fatalf("读取 Skill Agent 开关矩阵失败: %v", err)
	}
	bindingIndex := slices.IndexFunc(bindings, func(binding AgentSkillBinding) bool {
		return binding.AgentID == agentValue.AgentID
	})
	if bindingIndex < 0 || !bindings[bindingIndex].Available || !bindings[bindingIndex].Enabled {
		t.Fatalf("Skill Agent 开关矩阵状态不正确: %+v", bindings)
	}

	if err = service.UninstallSkill(ctx, agentValue.AgentID, "demo-skill"); err != nil {
		t.Fatalf("卸载 skill 失败: %v", err)
	}
	externalState, err = service.GetAgentSkillState(ctx, agentValue.AgentID, "demo-skill")
	if err != nil {
		t.Fatalf("读取已卸载外部 Skill 状态失败: %v", err)
	}
	if !externalState.Available || externalState.Installed {
		t.Fatalf("已卸载外部 Skill 状态不正确: %+v", externalState)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("再次读取 agent 技能失败: %v", err)
	}
	for _, item := range items {
		if item.Name == "demo-skill" && item.EnabledForAgent {
			t.Fatalf("停用后仍显示 enabled: %+v", item)
		}
	}
}

func TestBuiltinPlatformSkillStoresIDWithoutWorkspaceCopy(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	if err := workspacepkg.EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("初始化测试平台 Skill 失败: %v", err)
	}

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "平台 Skill 测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	systemBindings, err := service.ListSkillAgents(ctx, "imagegen")
	if err != nil {
		t.Fatalf("读取系统 Skill Agent 状态失败: %v", err)
	}
	systemBindingIndex := slices.IndexFunc(systemBindings, func(binding AgentSkillBinding) bool {
		return binding.AgentID == agentValue.AgentID
	})
	if systemBindingIndex < 0 ||
		systemBindings[systemBindingIndex].Available ||
		!systemBindings[systemBindingIndex].Enabled {
		t.Fatalf("系统托管 Skill 应显示已启用且不可手动切换: %+v", systemBindings)
	}
	enabled, err := service.InstallSkill(ctx, agentValue.AgentID, "ima-skill")
	if err != nil {
		t.Fatalf("通过兼容入口启用平台 builtin skill 失败: %v", err)
	}
	if !enabled.EnabledForAgent {
		t.Fatalf("平台 builtin skill 启用状态不正确: %+v", enabled)
	}
	if enabled.Version != "1.1.8" || enabled.CategoryKey != "content-docs" {
		t.Fatalf("IMA catalog 元数据不正确: %+v", enabled)
	}
	platformState, err := service.GetAgentSkillState(ctx, agentValue.AgentID, "ima-skill")
	if err != nil {
		t.Fatalf("读取平台 Skill 状态失败: %v", err)
	}
	if !platformState.Available || !platformState.Installed ||
		platformState.SourceType != sourceTypeBuiltin ||
		platformState.RuntimeVersion != agentValue.RuntimeVersion+1 {
		t.Fatalf("平台 Skill 状态快照不正确: %+v", platformState)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("重新读取 agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, "ima-skill") {
		t.Fatalf("Agent 未记录平台 Skill ID: %#v", reloaded.Options.SkillIDs)
	}
	if _, err = os.Stat(filepath.Join(reloaded.WorkspacePath, ".agents", "skills", "ima-skill")); !os.IsNotExist(err) {
		t.Fatalf("平台 Skill 不应复制到 workspace: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "ima-skill", "SKILL.md")); err != nil {
		t.Fatalf("平台全局 Skill 根缺少 IMA: %v", err)
	}
	disabled, err := service.SetAgentSkillEnabled(
		ctx,
		agentValue.AgentID,
		"ima-skill",
		false,
	)
	if err != nil {
		t.Fatalf("停用平台 Skill 失败: %v", err)
	}
	if disabled.EnabledForAgent {
		t.Fatalf("停用平台 Skill 后仍显示启用: %+v", disabled)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取停用平台 Skill 后的 Agent 失败: %v", err)
	}
	if slices.Contains(reloaded.Options.DisabledSkillIDs, "ima-skill") {
		t.Fatalf("平台 Skill 停用不应写入本地停用状态: %#v", reloaded.Options.DisabledSkillIDs)
	}
	if _, err = service.SetAgentSkillEnabled(ctx, agentValue.AgentID, "ima-skill", true); err != nil {
		t.Fatalf("重新启用平台 Skill 失败: %v", err)
	}
	if err = service.UninstallSkill(ctx, agentValue.AgentID, "ima-skill"); err != nil {
		t.Fatalf("卸载平台 builtin skill 失败: %v", err)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("卸载后重新读取 agent 失败: %v", err)
	}
	if slices.Contains(reloaded.Options.SkillIDs, "ima-skill") {
		t.Fatalf("卸载后仍保留平台 Skill ID: %#v", reloaded.Options.SkillIDs)
	}
	if slices.Contains(reloaded.Options.DisabledSkillIDs, "ima-skill") {
		t.Fatalf("卸载后不应残留平台 Skill 停用状态: %#v", reloaded.Options.DisabledSkillIDs)
	}
}

func TestAgentWorkspaceSkillIsPrivateAndEnabledByDefault(t *testing.T) {
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

	origin, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "技能作者"})
	if err != nil {
		t.Fatalf("创建来源 Agent 失败: %v", err)
	}
	skillRoot := filepath.Join(origin.WorkspacePath, ".agents", "skills", "agent-authored")
	if err = os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatalf("创建 Agent Skill 目录失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(`---
name: agent-authored
title: Agent Authored
description: 仅属于来源 Agent 的工作区 Skill
---

# agent-authored
`), 0o644); err != nil {
		t.Fatalf("写入 Agent Skill 失败: %v", err)
	}
	mainAgent, err := agentService.GetDefaultAgent(ctx)
	if err != nil {
		t.Fatalf("读取主 Agent 失败: %v", err)
	}
	if mainAgent.AgentID == origin.AgentID {
		t.Fatal("测试需要独立的来源 Agent 与目标 Agent")
	}

	originItems, err := service.GetAgentSkills(ctx, origin.AgentID)
	if err != nil {
		t.Fatalf("读取来源 Agent Skill 失败: %v", err)
	}
	originSkill, ok := findSkill(originItems, "agent-authored")
	if !ok || !originSkill.EnabledForAgent {
		t.Fatalf("工作区 Skill 应在所属 Agent 设置中默认启用: %+v", originSkill)
	}
	if _, err = service.GetSkillDetail(ctx, "agent-authored", ""); err == nil {
		t.Fatal("工作区 Skill 不应进入全局技能目录")
	}
	targetItems, err := service.GetAgentSkills(ctx, mainAgent.AgentID)
	if err != nil {
		t.Fatalf("读取其它 Agent Skill 失败: %v", err)
	}
	if containsSkill(targetItems, "agent-authored") {
		t.Fatalf("工作区 Skill 不应对其它 Agent 可见: %+v", targetItems)
	}
	if _, err = service.SetAgentSkillEnabled(
		ctx,
		mainAgent.AgentID,
		"agent-authored",
		true,
	); err == nil {
		t.Fatal("其它 Agent 不应能启用该工作区 Skill")
	}

	disabled, err := service.SetAgentSkillEnabled(
		ctx,
		origin.AgentID,
		"agent-authored",
		false,
	)
	if err != nil {
		t.Fatalf("停用所属 Agent 工作区 Skill 失败: %v", err)
	}
	if disabled.EnabledForAgent {
		t.Fatalf("停用后工作区 Skill 仍显示启用: %+v", disabled)
	}
	if _, err = os.Stat(filepath.Join(skillRoot, "SKILL.md")); err != nil {
		t.Fatalf("停用工作区 Skill 不应删除文件: %v", err)
	}
	if _, err = service.SetAgentSkillEnabled(ctx, origin.AgentID, "agent-authored", true); err != nil {
		t.Fatalf("重新启用所属 Agent 工作区 Skill 失败: %v", err)
	}
	reloadedItems, err := service.GetAgentSkills(ctx, origin.AgentID)
	if err != nil {
		t.Fatalf("重新读取来源 Agent Skill 失败: %v", err)
	}
	reloadedSkill, ok := findSkill(reloadedItems, "agent-authored")
	if !ok || !reloadedSkill.EnabledForAgent {
		t.Fatalf("重新启用后的工作区 Skill 状态不正确: %+v", reloadedSkill)
	}
}

func TestVersionedSkillMutationPreservesConcurrentAgentOptions(t *testing.T) {
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

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Skill CAS 测试助手"})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}
	staleVersion := agentValue.RuntimeVersion
	concurrentOptions := agentValue.Options
	concurrentOptions.PermissionMode = "plan"
	concurrent, err := agentService.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{
		Options: &concurrentOptions, ExpectedRuntimeVersion: &staleVersion,
	})
	if err != nil {
		t.Fatalf("并发更新 Agent options 失败: %v", err)
	}

	if _, err = service.InstallSkillAtVersion(
		ctx, agentValue.AgentID, "ima-skill", staleVersion,
	); !errors.Is(err, agentsvc.ErrRuntimeVersionConflict) {
		t.Fatalf("过期版本安装 error = %v, want ErrRuntimeVersionConflict", err)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取冲突后的 Agent 失败: %v", err)
	}
	if reloaded.Options.PermissionMode != "plan" || slices.Contains(reloaded.Options.SkillIDs, "ima-skill") {
		t.Fatalf("过期 Skill 安装覆盖并发 options: %+v", reloaded.Options)
	}

	installed, err := service.InstallSkillAtVersion(
		ctx, agentValue.AgentID, "ima-skill", concurrent.RuntimeVersion,
	)
	if err != nil {
		t.Fatalf("使用当前版本安装 Skill 失败: %v", err)
	}
	if !installed.EnabledForAgent {
		t.Fatalf("安装结果未标记 enabled_for_agent: %+v", installed)
	}
	afterInstall, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取安装后的 Agent 失败: %v", err)
	}
	if afterInstall.RuntimeVersion != concurrent.RuntimeVersion+1 ||
		afterInstall.Options.PermissionMode != "plan" ||
		!slices.Contains(afterInstall.Options.SkillIDs, "ima-skill") {
		t.Fatalf("版本化安装结果不正确: %+v", afterInstall)
	}

	uninstallVersion := afterInstall.RuntimeVersion
	nextOptions := afterInstall.Options
	nextOptions.AllowedTools = []string{"Read"}
	concurrent, err = agentService.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{
		Options: &nextOptions, ExpectedRuntimeVersion: &uninstallVersion,
	})
	if err != nil {
		t.Fatalf("第二次并发更新 Agent options 失败: %v", err)
	}
	if err = service.UninstallSkillAtVersion(
		ctx, agentValue.AgentID, "ima-skill", uninstallVersion,
	); !errors.Is(err, agentsvc.ErrRuntimeVersionConflict) {
		t.Fatalf("过期版本卸载 error = %v, want ErrRuntimeVersionConflict", err)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取卸载冲突后的 Agent 失败: %v", err)
	}
	if !slices.Equal(reloaded.Options.AllowedTools, []string{"Read"}) ||
		!slices.Contains(reloaded.Options.SkillIDs, "ima-skill") {
		t.Fatalf("过期 Skill 卸载覆盖并发 options: %+v", reloaded.Options)
	}
	if err = service.UninstallSkillAtVersion(
		ctx, agentValue.AgentID, "ima-skill", concurrent.RuntimeVersion,
	); err != nil {
		t.Fatalf("使用当前版本卸载 Skill 失败: %v", err)
	}
	afterUninstall, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取卸载后的 Agent 失败: %v", err)
	}
	if afterUninstall.RuntimeVersion != concurrent.RuntimeVersion+1 ||
		!slices.Equal(afterUninstall.Options.AllowedTools, []string{"Read"}) ||
		slices.Contains(afterUninstall.Options.SkillIDs, "ima-skill") {
		t.Fatalf("版本化卸载结果不正确: %+v", afterUninstall)
	}
}

func TestGlobalSkillReferencesKeepSourceClassification(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	platformRecord := catalogRecord{
		Detail: Detail{Info: Info{
			Name:       "ima-skill",
			SourceType: sourceTypeBuiltin,
			SourceKind: sourceKindNexusPlatform,
		}},
		SourcePath: filepath.Join(appfs.Root(), "skills", "ima-skill"),
	}
	if reference := skillReference(platformRecord); reference != "ima-skill" {
		t.Fatalf("平台 Skill 引用 = %q, want ima-skill", reference)
	}
	if got := builtinSourceKind(filepath.Join(appfs.Root(), "skills"), filepath.Join(appfs.Root(), "skills")); got != sourceKindNexusPlatform {
		t.Fatalf("产品 skills 目录来源 = %q, want %q", got, sourceKindNexusPlatform)
	}
	if got := builtinOriginKind(sourceKindNexusPlatform); got != originKindBuiltin {
		t.Fatalf("平台 Skill 来源类型 = %q, want %q", got, originKindBuiltin)
	}

	userGlobalRecord := catalogRecord{
		Detail: Detail{Info: Info{
			Name:       "user-skill",
			SourceType: sourceTypeBuiltin,
			SourceKind: sourceKindUserGlobal,
		}},
		SourcePath: filepath.Join(t.TempDir(), "user-skill"),
	}
	if reference := skillReference(userGlobalRecord); reference != "user-skill" {
		t.Fatalf("用户全局 Skill 引用 = %q, want user-skill", reference)
	}
	if got := builtinSourceKind(filepath.Join(t.TempDir(), "skills"), filepath.Join(appfs.Root(), "skills")); got != sourceKindUserGlobal {
		t.Fatalf("用户全局 Skill 来源 = %q, want %q", got, sourceKindUserGlobal)
	}
	if got := builtinOriginKind(sourceKindUserGlobal); got != originKindUserImport {
		t.Fatalf("用户全局 Skill 来源类型 = %q, want %q", got, originKindUserImport)
	}
	roots := builtinSearchRoots(appfs.Root())
	agentSkillsRoot := filepath.Join(appfs.HostSkillRoot(), ".agents", "skills")
	if !slices.Contains(roots, agentSkillsRoot) {
		t.Fatalf("全局目录未发现受管宿主 Skill 根: %s", agentSkillsRoot)
	}
	for _, unsupportedRoot := range []string{
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".cc-switch", "skills"),
	} {
		if slices.Contains(roots, unsupportedRoot) {
			t.Fatalf("全局目录不应扫描其他宿主 Skill 根: %s", unsupportedRoot)
		}
	}
}

func TestSkillSourceIdentityBindsScopeAndContent(t *testing.T) {
	base := catalogRecord{Detail: Detail{
		Info: Info{
			Name:         "identity-skill",
			Title:        "Identity Skill",
			Scope:        scopeAny,
			SourceType:   sourceTypeWorkspace,
			SourceRef:    "/workspace/.agents/skills/identity-skill",
			StorageScope: storageScopeAgent,
			Version:      "workspace",
		},
		ReadmeMarkdown: "# version one",
	}}
	first := selectionInfo(base)
	changed := base
	changed.Detail.ReadmeMarkdown = "# version two"
	second := selectionInfo(changed)
	global := base
	global.Detail.SourceType = sourceTypeBuiltin
	global.Detail.StorageScope = storageScopeUserGlobal
	third := selectionInfo(global)
	if first.SourceIdentity == "" ||
		first.SourceIdentity == second.SourceIdentity ||
		first.SourceIdentity == third.SourceIdentity ||
		first.TargetScope != AgentSkillTargetWorkspace ||
		third.TargetScope != AgentSkillTargetGlobalLibrary {
		t.Fatalf("source identity 未绑定内容与作用域: first=%+v second=%+v global=%+v", first, second, third)
	}
}

func TestAuthenticatedSkillCatalogOnlyScansUserGlobalRootInDesktopMode(t *testing.T) {
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: true})
	t.Setenv("NEXUS_APP_MODE", "")
	roots := builtinSearchRootsForContext(ctx, appfs.Root(), "")
	if len(roots) != 1 || filepath.Clean(roots[0]) != filepath.Join(appfs.Root(), "skills") {
		t.Fatalf("认证服务不应扫描宿主用户 Skill 根: %#v", roots)
	}

	t.Setenv("NEXUS_APP_MODE", "desktop")
	roots = builtinSearchRootsForContext(ctx, appfs.Root(), "desktop")
	if !slices.Contains(roots, filepath.Join(appfs.HostSkillRoot(), ".agents", "skills")) {
		t.Fatalf("桌面模式应读取受管宿主 Skill 根: %#v", roots)
	}
}

func TestUserGlobalSkillUsesGlobalAgentBinding(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	cfg.AppMode = "desktop"
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("NEXUS_APP_MODE", "desktop")
	writeTestSkillDir(
		t,
		filepath.Join(home, ".agents", "skills", "host-global-skill"),
		"host-global-skill",
		"用户全局 Skill",
		false,
	)
	if err = workspacepkg.EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 投影失败: %v", err)
	}

	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "用户全局 Skill 测试助手",
	})
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}
	enabled, err := service.SetAgentSkillEnabledInScope(
		ctx,
		agentValue.AgentID,
		"host-global-skill",
		true,
		AgentSkillTargetGlobalLibrary,
	)
	if err != nil {
		t.Fatalf("启用用户全局 Skill 失败: %v", err)
	}
	if !enabled.EnabledForAgent ||
		enabled.SourceKind != sourceKindUserGlobal ||
		enabled.StorageScope != storageScopeUserGlobal {
		t.Fatalf("用户全局 Skill 启用状态不正确: %+v", enabled)
	}
	workspaceCopy := filepath.Join(
		agentValue.WorkspacePath,
		".agents",
		"skills",
		"host-global-skill",
	)
	if _, err = os.Stat(workspaceCopy); !os.IsNotExist(err) {
		t.Fatalf("用户全局 Skill 不应复制到 Agent workspace: %v", err)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取启用后的 Agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, "host-global-skill") {
		t.Fatalf("用户全局 Skill 未写入 Agent 全局绑定: %#v", reloaded.Options.SkillIDs)
	}

	globalItems, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取全局 Skill 目录失败: %v", err)
	}
	globalItem, ok := findSkill(globalItems, "host-global-skill")
	if !ok || globalItem.EnabledAgentCount != 1 {
		t.Fatalf("用户全局 Skill 使用数不正确: %+v", globalItem)
	}

	disabled, err := service.SetAgentSkillEnabledInScope(
		ctx,
		agentValue.AgentID,
		"host-global-skill",
		false,
		AgentSkillTargetGlobalLibrary,
	)
	if err != nil {
		t.Fatalf("停用用户全局 Skill 失败: %v", err)
	}
	if disabled.EnabledForAgent {
		t.Fatalf("停用后用户全局 Skill 仍显示启用: %+v", disabled)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取停用后的 Agent 失败: %v", err)
	}
	if slices.Contains(reloaded.Options.SkillIDs, "host-global-skill") {
		t.Fatalf("停用后仍保留用户全局 Skill 绑定: %#v", reloaded.Options.SkillIDs)
	}
	if slices.Contains(reloaded.Options.DisabledSkillIDs, "host-global-skill") {
		t.Fatalf("全局 Skill 停用状态不应写入本地停用列表: %#v", reloaded.Options.DisabledSkillIDs)
	}
}

func TestHostSkillCanonicalNameUsesDirectoryName(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(filepath.Dir(cfg.WorkspacePath), "home")
	sourceDir := filepath.Join(home, ".agents", "skills", "directory-name")
	writeTestSkillDir(
		t,
		sourceDir,
		"frontmatter-name",
		"Frontmatter 标题",
		false,
	)
	if err := workspacepkg.EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 投影失败: %v", err)
	}
	writeTestSkillDir(t, sourceDir, "frontmatter-name", "未同步的宿主标题", false)

	items, err := NewService(cfg, nil, nil).ListSkills(context.Background(), Query{})
	if err != nil {
		t.Fatalf("读取宿主 Skill 目录失败: %v", err)
	}
	item, ok := findSkill(items, "directory-name")
	if !ok {
		t.Fatalf("目录名 canonical Skill 缺失: %+v", items)
	}
	if item.Title != "Frontmatter 标题" || item.SourceKind != sourceKindUserGlobal {
		t.Fatalf("宿主 Skill 元数据投影不正确: %+v", item)
	}
	expectedSource := filepath.Join(appfs.HostSkillRoot(), ".agents", "skills", "directory-name")
	if filepath.Clean(item.SourceRef) != filepath.Clean(expectedSource) {
		t.Fatalf("宿主 Catalog 未读取受管快照: %q, want %q", item.SourceRef, expectedSource)
	}
	if _, exists := findSkill(items, "frontmatter-name"); exists {
		t.Fatalf("frontmatter name 不应替代目录 canonical name: %+v", items)
	}
	if err = workspacepkg.EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("刷新宿主 Skill 投影失败: %v", err)
	}
	items, err = NewService(cfg, nil, nil).ListSkills(context.Background(), Query{})
	if err != nil {
		t.Fatalf("读取刷新后的宿主 Skill 目录失败: %v", err)
	}
	item, ok = findSkill(items, "directory-name")
	if !ok || item.Title != "未同步的宿主标题" {
		t.Fatalf("宿主快照显式刷新后未更新: %+v", item)
	}
}

func TestOwnerGlobalSkillCanonicalNameUsesDirectoryName(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	skillRoot := filepath.Join(
		workspacepkg.UserSkillDiscoveryRoot(cfg, authctx.SystemUserID),
		"owner-directory-name",
	)
	writeTestSkillDir(
		t,
		skillRoot,
		"owner-frontmatter-name",
		"Owner Frontmatter 标题",
		true,
	)

	items, err := NewService(cfg, nil, nil).ListSkills(context.Background(), Query{})
	if err != nil {
		t.Fatalf("读取 owner 全局 Skill 目录失败: %v", err)
	}
	item, ok := findSkill(items, "owner-directory-name")
	if !ok {
		t.Fatalf("owner 目录名 canonical Skill 缺失: %+v", items)
	}
	if item.Title != "Owner Frontmatter 标题" || item.StorageScope != storageScopeUserGlobal {
		t.Fatalf("owner 全局 Skill 元数据投影不正确: %+v", item)
	}
	if _, exists := findSkill(items, "owner-frontmatter-name"); exists {
		t.Fatalf("owner manifest/frontmatter 不应改写目录 canonical name: %+v", items)
	}
}

func TestAgentWorkspaceSkillShadowsSameNamedUserGlobalSkill(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	cfg.AppMode = "desktop"
	migrateSkillsSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := workspacepkg.NewService(cfg, agentService)
	service := NewService(cfg, agentService, workspaceService)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("NEXUS_APP_MODE", "desktop")
	hostSkillRoot := filepath.Join(home, ".agents", "skills", "same-name-skill")
	writeTestSkillDir(t, hostSkillRoot, "same-name-skill", "用户全局版本", false)
	if err = workspacepkg.EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 投影失败: %v", err)
	}

	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name: "本地 Skill 测试助手",
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	workspaceSkillRoot := filepath.Join(
		agentValue.WorkspacePath,
		".agents",
		"skills",
		"same-name-skill",
	)
	writeTestSkillDir(t, workspaceSkillRoot, "same-name-skill", "Agent 本地版本", false)

	ctx := context.Background()
	items, err := service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 Agent Skill 失败: %v", err)
	}
	item, ok := findSkill(items, "same-name-skill")
	if !ok {
		t.Fatalf("未找到 Agent 本地 Skill: %+v", items)
	}
	if item.SourceType != sourceTypeWorkspace ||
		item.StorageScope != storageScopeAgent ||
		item.TargetScope != AgentSkillTargetWorkspace ||
		item.SourceIdentity == "" ||
		item.Title != "Agent 本地版本" ||
		!item.EnabledForAgent {
		t.Fatalf("同名 Skill 应按当前 Agent 本地来源投影: %+v", item)
	}

	globalItems, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取全局 Skill 目录失败: %v", err)
	}
	globalItem, ok := findSkill(globalItems, "same-name-skill")
	if !ok {
		t.Fatalf("全局目录未找到用户全局 Skill: %+v", globalItems)
	}
	if globalItem.SourceKind != sourceKindUserGlobal ||
		globalItem.TargetScope != AgentSkillTargetGlobalLibrary ||
		globalItem.SourceIdentity == "" ||
		globalItem.SourceIdentity == item.SourceIdentity ||
		globalItem.Title != "用户全局版本" ||
		globalItem.EnabledAgentCount != 0 {
		t.Fatalf("Agent 本地同名 Skill 不应污染全局投影: %+v", globalItem)
	}

	bindings, err := service.ListSkillAgents(ctx, "same-name-skill")
	if err != nil {
		t.Fatalf("读取全局 Skill 的 Agent 矩阵失败: %v", err)
	}
	bindingIndex := slices.IndexFunc(bindings, func(binding AgentSkillBinding) bool {
		return binding.AgentID == agentValue.AgentID
	})
	if bindingIndex < 0 || bindings[bindingIndex].Enabled {
		t.Fatalf("Agent 本地同名 Skill 不应启用全局 Skill: %+v", bindings)
	}

	globalState, err := service.GetAgentSkillStateInScope(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		AgentSkillTargetGlobalLibrary,
	)
	if err != nil {
		t.Fatalf("读取同名全局 Skill 来源状态失败: %v", err)
	}
	localState, err := service.GetAgentSkillStateInScope(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		AgentSkillTargetWorkspace,
	)
	if err != nil {
		t.Fatalf("读取同名本地 Skill 来源状态失败: %v", err)
	}
	if globalState.SourceIdentity == "" ||
		localState.SourceIdentity == "" ||
		globalState.SourceIdentity == localState.SourceIdentity ||
		globalState.TargetScope != AgentSkillTargetGlobalLibrary ||
		localState.TargetScope != AgentSkillTargetWorkspace {
		t.Fatalf("同名 Skill 来源身份未按 scope 区分: global=%+v local=%+v", globalState, localState)
	}
	if _, err = service.SetAgentSkillEnabledInScopeAtVersion(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		true,
		AgentSkillTargetGlobalLibrary,
		"skill-source:stale",
		globalState.RuntimeVersion,
	); err == nil {
		t.Fatal("错误 source_identity 不得更新 Skill 选择")
	}
	globalEnabled, err := service.SetAgentSkillEnabledInScopeAtVersion(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		true,
		AgentSkillTargetGlobalLibrary,
		globalState.SourceIdentity,
		globalState.RuntimeVersion,
	)
	if err != nil {
		t.Fatalf("启用同名全局 Skill 失败: %v", err)
	}
	if !globalEnabled.EnabledForAgent {
		t.Fatalf("同名全局 Skill 未返回启用状态: %+v", globalEnabled)
	}
	reloaded, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取启用同名全局 Skill 后的 Agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, "same-name-skill") {
		t.Fatalf("同名全局 Skill 未独立记录绑定: %#v", reloaded.Options.SkillIDs)
	}
	localState, err = service.GetAgentSkillStateInScope(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		AgentSkillTargetWorkspace,
	)
	if err != nil {
		t.Fatalf("重新读取同名本地 Skill 来源状态失败: %v", err)
	}
	localDisabled, err := service.SetAgentSkillEnabledInScopeAtVersion(
		ctx,
		agentValue.AgentID,
		"same-name-skill",
		false,
		AgentSkillTargetWorkspace,
		localState.SourceIdentity,
		localState.RuntimeVersion,
	)
	if err != nil {
		t.Fatalf("停用同名 Agent 本地 Skill 失败: %v", err)
	}
	if localDisabled.EnabledForAgent {
		t.Fatalf("同名 Agent 本地 Skill 停用状态不正确: %+v", localDisabled)
	}
	reloaded, err = agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取停用同名本地 Skill 后的 Agent 失败: %v", err)
	}
	if !slices.Contains(reloaded.Options.SkillIDs, "same-name-skill") {
		t.Fatalf("停用本地 Skill 不应移除同名全局绑定: %#v", reloaded.Options.SkillIDs)
	}
	if !slices.Contains(reloaded.Options.DisabledSkillIDs, "same-name-skill") {
		t.Fatalf("停用本地 Skill 未写入 disabled_skill_ids: %#v", reloaded.Options.DisabledSkillIDs)
	}
	if _, err = os.Stat(workspaceSkillRoot); err != nil {
		t.Fatalf("版本化停用本地 Skill 不得删除 workspace 文件: %v", err)
	}
	items, err = service.GetAgentSkills(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("重新读取 Agent Skill 失败: %v", err)
	}
	item, ok = findSkill(items, "same-name-skill")
	if !ok || item.SourceType != sourceTypeWorkspace || item.EnabledForAgent {
		t.Fatalf("Agent 设置页应显示同名本地 Skill 已停用: %+v", item)
	}
	bindings, err = service.ListSkillAgents(ctx, "same-name-skill")
	if err != nil {
		t.Fatalf("重新读取同名全局 Skill 矩阵失败: %v", err)
	}
	bindingIndex = slices.IndexFunc(bindings, func(binding AgentSkillBinding) bool {
		return binding.AgentID == agentValue.AgentID
	})
	if bindingIndex < 0 || !bindings[bindingIndex].Enabled {
		t.Fatalf("本地 Skill 状态不应污染同名全局绑定: %+v", bindings)
	}
}

func TestUpdateSingleSkillUsesSharedUserSource(t *testing.T) {
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
	for _, agentValue := range []protocol.Agent{*failingAgent, *successAgent} {
		reloaded, getErr := agentService.GetAgent(ctx, agentValue.AgentID)
		if getErr != nil {
			t.Fatalf("读取已绑定 Agent 失败: %v", getErr)
		}
		if !slices.Contains(reloaded.Options.SkillIDs, protocol.BuildExternalSkillReference("git-skill")) {
			t.Fatalf("外部 skill 应保存为用户级引用: %#v", reloaded.Options.SkillIDs)
		}
		if _, statErr := os.Stat(filepath.Join(reloaded.WorkspacePath, ".agents", "skills", "git-skill")); !os.IsNotExist(statErr) {
			t.Fatalf("外部 skill 不应复制到 workspace: %v", statErr)
		}
	}

	activeRepo = repoV2
	activeCommit = "commit-v2"
	detail, err := service.UpdateSingleSkill(ctx, "git-skill")
	if err != nil {
		t.Fatalf("更新 Git skill 失败: %v", err)
	}
	if len(detail.DeployFailures) != 0 {
		t.Fatalf("全局源更新不应产生 workspace 部署失败: %+v", detail.DeployFailures)
	}
	if len(detail.DeploySuccesses) != 2 {
		t.Fatalf("应返回两个已绑定 Agent 的全局源影响结果: %+v", detail.DeploySuccesses)
	}
	globalSkillPath := filepath.Join(workspacepkg.UserSkillDiscoveryRoot(cfg, authctx.SystemUserID), "git-skill", "SKILL.md")
	payload, err := os.ReadFile(globalSkillPath)
	if err != nil {
		t.Fatalf("读取更新后的用户级 skill 失败: %v", err)
	}
	if !strings.Contains(string(payload), "Git Skill v2") {
		t.Fatalf("用户级源未随库更新: %s", payload)
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

func newSkillsTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("USERPROFILE", filepath.Join(root, "home"))
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
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
	handlertest.MigrateSQLiteFromDir(t, databaseURL, skillsTestMigrationDir(t))
}

func skillsTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
