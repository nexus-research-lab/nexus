package agent_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"

	_ "modernc.org/sqlite"
)

func TestServiceListAgentsUsesSystemScopeWhenAuthIsDisabled(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)

	singleUserContext := authctx.WithState(context.Background(), authctx.State{
		AuthRequired: false,
		UserCount:    2,
	})
	if _, err := service.ListAgents(singleUserContext); err != nil {
		t.Fatalf("初始化 system agent 失败: %v", err)
	}
	userContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-b"})
	if _, err := service.CreateAgent(userContext, protocol.CreateRequest{Name: "用户 B 助手"}); err != nil {
		t.Fatalf("创建用户 B agent 失败: %v", err)
	}

	items, err := service.ListAgents(singleUserContext)
	if err != nil {
		t.Fatalf("单用户作用域列出 agent 失败: %v", err)
	}
	if len(items) != 1 || items[0].OwnerUserID != authctx.SystemUserID {
		t.Fatalf("认证关闭时不应返回其他 owner agent: %+v", items)
	}
}

func TestServiceManagesSymmetricAgentContacts(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "contact-owner", Role: authctx.RoleOwner,
	})
	amy, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatal(err)
	}
	devin, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "Devin"})
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.AddAgentContact(ctx, amy.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: devin.AgentID, Alias: "开发搭档",
	})
	if err != nil {
		t.Fatalf("AddAgentContact() error = %v", err)
	}
	if created.ContactAgentID != devin.AgentID || created.Alias != "开发搭档" {
		t.Fatalf("created contact = %+v", created)
	}
	reverse, err := service.ListAgentContacts(ctx, devin.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reverse) != 1 || reverse[0].ContactAgentID != amy.AgentID || reverse[0].Alias != "" {
		t.Fatalf("reverse contacts = %+v", reverse)
	}
	if _, err = service.AddAgentContact(ctx, devin.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: amy.AgentID, Alias: "产品搭档",
	}); err != nil {
		t.Fatal(err)
	}
	forward, err := service.ListAgentContacts(ctx, amy.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(forward) != 1 || forward[0].Alias != "开发搭档" {
		t.Fatalf("peer alias update changed owner alias: %+v", forward)
	}
	mainAgent, err := service.GetDefaultAgent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddAgentContact(ctx, mainAgent.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: amy.AgentID,
	}); !errors.Is(err, agentpkg.ErrAgentContactUnsupported) {
		t.Fatalf("main Agent contact error = %v", err)
	}
	if err = service.DeleteAgentContact(ctx, devin.AgentID, amy.AgentID); err != nil {
		t.Fatal(err)
	}
	for _, agentID := range []string{amy.AgentID, devin.AgentID} {
		items, listErr := service.ListAgentContacts(ctx, agentID)
		if listErr != nil || len(items) != 0 {
			t.Fatalf("contacts after delete for %s = %+v, err = %v", agentID, items, listErr)
		}
	}
}

func TestServiceGetAgentRejectsOwnerWorkspaceSymlink(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ownerAContext := authctx.WithPrincipal(
		context.Background(),
		&authctx.Principal{UserID: "user-a"},
	)
	ownerBContext := authctx.WithPrincipal(
		context.Background(),
		&authctx.Principal{UserID: "user-b"},
	)
	ownerAAgent, err := service.CreateAgent(
		ownerAContext,
		protocol.CreateRequest{Name: "owner-a-agent"},
	)
	if err != nil {
		t.Fatal(err)
	}
	ownerBAgent, err := service.CreateAgent(
		ownerBContext,
		protocol.CreateRequest{Name: "owner-b-agent"},
	)
	if err != nil {
		t.Fatal(err)
	}
	foreignSettingsPath := filepath.Join(ownerBAgent.WorkspacePath, ".nexus", "settings.json")
	before, err := os.ReadFile(foreignSettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(ownerAAgent.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(ownerBAgent.WorkspacePath, ownerAAgent.WorkspacePath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err = service.GetAgent(ownerAContext, ownerAAgent.AgentID); !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("Agent 查询不能借 workspace symlink 写入另一 owner: %v", err)
	}
	after, err := os.ReadFile(foreignSettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("被链接 owner 的 runtime settings 不应发生变化")
	}
}

func TestServiceBootstrapsMainAgentAndCreatesAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)

	ctx := context.Background()

	items, err := service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("列出主智能体失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("主智能体初始化数量不正确: got=%d", len(items))
	}
	if items[0].AgentID != cfg.DefaultAgentID {
		t.Fatalf("主智能体 ID 不匹配: got=%s want=%s", items[0].AgentID, cfg.DefaultAgentID)
	}
	if items[0].Avatar != "nexus" {
		t.Fatalf("主智能体应使用 Nexus 默认头像: got=%s", items[0].Avatar)
	}
	if items[0].Options.Provider != "" {
		t.Fatalf("主智能体应跟随默认 provider，不应写死显式 provider: %+v", items[0].Options)
	}
	if items[0].Options.PermissionMode != protocol.DefaultAgentPermissionMode {
		t.Fatalf("主智能体默认权限应自动接受编辑: %+v", items[0].Options)
	}
	if len(items[0].Options.AllowedTools) != 0 {
		t.Fatalf("主智能体默认不应预授权工具: %+v", items[0].Options.AllowedTools)
	}
	updatedMain, err := service.UpdateAgent(ctx, items[0].AgentID, protocol.UpdateRequest{
		Options: &protocol.Options{Provider: "stale-provider", Model: "stale-model"},
	})
	if err != nil {
		t.Fatalf("更新主智能体失败: %v", err)
	}
	if updatedMain.Options.Provider != "" || updatedMain.Options.Model != "" {
		t.Fatalf("主智能体模型应始终跟随全局默认: %+v", updatedMain.Options)
	}
	assertRuntimeEmotionStateFile(t, items[0].WorkspacePath)

	validation, err := service.ValidateName(ctx, "测试助手", "")
	if err != nil {
		t.Fatalf("校验名称失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("名称应该可用: %+v", validation)
	}

	created, err := service.CreateAgent(ctx, protocol.CreateRequest{
		Name:        "测试助手",
		Description: "首个集成测试 agent",
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.AgentID == "" {
		t.Fatal("创建后的 agent_id 不能为空")
	}
	if created.Avatar == "" {
		t.Fatal("创建 Agent 时应自动分配头像")
	}
	if created.Options.PermissionMode != protocol.DefaultAgentPermissionMode {
		t.Fatalf("新 Agent 默认权限应自动接受编辑: %+v", created.Options)
	}
	if _, err = os.Stat(created.WorkspacePath); err != nil {
		t.Fatalf("workspace 目录未创建: %v", err)
	}
	assertRuntimeEmotionStateFile(t, created.WorkspacePath)
	profileTemplate, err := os.ReadFile(filepath.Join(created.WorkspacePath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("创建 Agent 时应立即写入默认行为模板: %v", err)
	}
	if !strings.Contains(string(profileTemplate), "## Role") {
		t.Fatalf("默认行为模板内容不正确: %s", profileTemplate)
	}
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-a"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-a 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(created.WorkspacePath, ".agents", "skills", "skill-a", "SKILL.md"), []byte("# skill-a\n"), 0o644); err != nil {
		t.Fatalf("写入测试 skill-a 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(created.WorkspacePath, ".claude", "skills", "skill-b"), 0o755); err != nil {
		t.Fatalf("创建测试 skill-b 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(created.WorkspacePath, ".claude", "skills", "skill-b", "SKILL.md"), []byte("# skill-b\n"), 0o644); err != nil {
		t.Fatalf("写入测试 skill-b 失败: %v", err)
	}

	loaded, err := service.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatalf("读取 agent 失败: %v", err)
	}
	if loaded.SkillsCount != 6 {
		t.Fatalf("skills_count 不正确: got=%d want=6", loaded.SkillsCount)
	}

	items, err = service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("再次列出 agent 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("agent 数量不正确: got=%d want=2", len(items))
	}
	for _, item := range items {
		if item.AgentID == created.AgentID && item.SkillsCount != 6 {
			t.Fatalf("list_agents skills_count 不正确: got=%d want=6", item.SkillsCount)
		}
	}

	validation, err = service.ValidateName(ctx, "测试助手", "")
	if err != nil {
		t.Fatalf("重复名称校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("重复名称应只作为展示名并允许复用: %+v", validation)
	}
}

func TestServiceRetriesMainAgentWorkspaceLifecycleBeforePersisting(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db := newAgentTestService(t, cfg)
	manager := &recordingWorkspaceManager{initializeErr: errors.New("initialize failed")}
	service.SetWorkspaceManager(manager)
	ctx := context.Background()

	if _, err := service.ListAgents(ctx); err == nil {
		t.Fatal("workspace 初始化失败时不应提交主 Agent")
	}
	assertNoRowsForAgent(t, db, "agents", "id", cfg.DefaultAgentID)
	if len(manager.initialized) != 1 || !manager.initialized[0].IsMain {
		t.Fatalf("主 Agent 应进入 workspace 生命周期: %+v", manager.initialized)
	}

	manager.initializeErr = nil
	items, err := service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("重试主 Agent workspace 生命周期失败: %v", err)
	}
	if len(items) != 1 || items[0].AgentID != cfg.DefaultAgentID {
		t.Fatalf("主 Agent 重试后未正确落库: %+v", items)
	}
	if len(manager.initialized) != 2 {
		t.Fatalf("workspace 初始化调用次数 = %d, want 2", len(manager.initialized))
	}
	initialized := manager.initialized[1]
	if initialized.AgentID != items[0].AgentID ||
		initialized.OwnerUserID != items[0].OwnerUserID ||
		initialized.WorkspacePath != items[0].WorkspacePath ||
		!initialized.IsMain || initialized.CreatedAt.IsZero() {
		t.Fatalf("workspace 生命周期收到的主 Agent 身份不完整: %+v", initialized)
	}

	if _, err = service.GetDefaultAgent(ctx); err != nil {
		t.Fatalf("再次读取主 Agent 失败: %v", err)
	}
	if len(manager.initialized) != 2 {
		t.Fatalf("普通读取不应重复初始化 workspace: calls=%d", len(manager.initialized))
	}
}

func TestServiceRejectsMainAgentDeletionBeforeCoordinator(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	ctx := context.Background()
	if _, err = service.ListAgents(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}
	if err = service.DeleteAgent(ctx, cfg.DefaultAgentID); err == nil ||
		!strings.Contains(err.Error(), "主智能体不可删除") {
		t.Fatalf("主智能体删除必须在协调前拒绝: %v", err)
	}
	mainAgent, err := service.GetAgent(ctx, cfg.DefaultAgentID)
	if err != nil || mainAgent == nil || !mainAgent.IsMain {
		t.Fatalf("拒绝删除后主智能体必须保留: agent=%+v err=%v", mainAgent, err)
	}
}

func TestCreateAgentPersistsCustomizedProfileTemplate(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)
	const customTemplate = "## Role\n\n- Purpose: 负责发布前质量审查\n"
	created, err := service.CreateAgent(context.Background(), protocol.CreateRequest{
		Name:            "质量审查助手",
		ProfileTemplate: customTemplate,
	})
	if err != nil {
		t.Fatalf("创建自定义模板 Agent 失败: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(created.WorkspacePath, "AGENTS.md"))
	if err != nil {
		t.Fatalf("读取自定义行为模板失败: %v", err)
	}
	if string(content) != customTemplate {
		t.Fatalf("自定义行为模板未原样落盘: got=%q want=%q", content, customTemplate)
	}
}

func TestServiceProjectsNexusAvatarForLegacyMainAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db := newAgentTestService(t, cfg)

	ctx := context.Background()
	if _, err := service.ListAgents(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE agents SET avatar = NULL WHERE id = ?`, cfg.DefaultAgentID); err != nil {
		t.Fatalf("模拟旧主智能体头像数据失败: %v", err)
	}

	loaded, err := service.GetDefaultAgent(ctx)
	if err != nil {
		t.Fatalf("读取旧主智能体失败: %v", err)
	}
	if loaded.Avatar != "nexus" {
		t.Fatalf("旧主智能体应投影 Nexus 默认头像: got=%s", loaded.Avatar)
	}
}

func TestServicePersistsAgentRuntimeProviderModel(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)

	ctx := context.Background()
	maxTurns := 6
	maxThinkingTokens := 2048
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{
		Name: "runtime-agent",
		Options: &protocol.Options{
			Provider:          "glm",
			Model:             "glm-5.1",
			PermissionMode:    "default",
			AllowedTools:      []string{"Read"},
			DisallowedTools:   []string{"Write"},
			MaxTurns:          &maxTurns,
			MaxThinkingTokens: &maxThinkingTokens,
			MCPServers:        map[string]any{"local": map[string]any{"command": "nexus-mcp"}},
			SettingSources:    []string{"project"},
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.Options.Provider != "glm" || created.Options.Model != "glm-5.1" {
		t.Fatalf("runtime provider/model 未持久化: %+v", created.Options)
	}

	nextName := "runtime-agent"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Name: &nextName,
		Options: &protocol.Options{
			Provider:        "kimi-code",
			Model:           "kimi-for-coding",
			PermissionMode:  "bypassPermissions",
			AllowedTools:    []string{"Read", "Edit"},
			DisallowedTools: []string{},
			SettingSources:  []string{"project"},
		},
	})
	if err != nil {
		t.Fatalf("更新 agent runtime 失败: %v", err)
	}
	if updated.Options.Provider != "kimi-code" || updated.Options.Model != "kimi-for-coding" {
		t.Fatalf("runtime provider/model 更新后未持久化: %+v", updated.Options)
	}
	if updated.Options.MaxTurns == nil || *updated.Options.MaxTurns != maxTurns {
		t.Fatalf("未提交 max_turns 时应保留原值: %+v", updated.Options)
	}
	if updated.Options.MaxThinkingTokens == nil || *updated.Options.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("未提交 max_thinking_tokens 时应保留原值: %+v", updated.Options)
	}
	if _, ok := updated.Options.MCPServers["local"]; !ok {
		t.Fatalf("未提交 mcp_servers 时应保留原值: %+v", updated.Options.MCPServers)
	}
}

func TestServiceUsesRuntimeVersionForCompareAndSwap(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "versioned-agent"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if created.RuntimeVersion != 1 {
		t.Fatalf("初始 runtime_version = %d, want 1", created.RuntimeVersion)
	}

	expectedVersion := created.RuntimeVersion
	options := created.Options
	options.PermissionMode = "plan"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Options:                &options,
		ExpectedRuntimeVersion: &expectedVersion,
	})
	if err != nil {
		t.Fatalf("使用当前版本更新 agent 失败: %v", err)
	}
	if updated.RuntimeVersion != 2 {
		t.Fatalf("更新后 runtime_version = %d, want 2", updated.RuntimeVersion)
	}

	staleName := "stale-name"
	staleOptions := updated.Options
	staleOptions.PermissionMode = "bypassPermissions"
	if _, err = service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Name:                   &staleName,
		Options:                &staleOptions,
		ExpectedRuntimeVersion: &expectedVersion,
	}); !errors.Is(err, agentpkg.ErrRuntimeVersionConflict) {
		t.Fatalf("使用过期版本更新 error = %v, want ErrRuntimeVersionConflict", err)
	}

	current, err := service.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatalf("重新读取 agent 失败: %v", err)
	}
	if current.RuntimeVersion != 2 || current.Name != created.Name || current.Options.PermissionMode != "plan" {
		t.Fatalf("过期更新未整体回滚: %+v", current)
	}

	unconditionalOptions := current.Options
	unconditionalOptions.PermissionMode = "default"
	unconditional, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{
		Options: &unconditionalOptions,
	})
	if err != nil {
		t.Fatalf("无 expected version 更新失败: %v", err)
	}
	if unconditional.RuntimeVersion != 3 {
		t.Fatalf("无条件更新后 runtime_version = %d, want 3", unconditional.RuntimeVersion)
	}
}

func TestServiceDeleteAtVersionRejectsStalePlanBeforeWorkspaceCleanup(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "versioned-delete-agent"})
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(created.WorkspacePath, "must-survive-stale-delete.txt")
	if err = os.WriteFile(markerPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	nextName := "versioned-delete-agent-updated"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatal(err)
	}

	if err = service.DeleteAgentAtVersion(
		ctx,
		created.AgentID,
		created.RuntimeVersion,
	); !errors.Is(err, agentpkg.ErrRuntimeVersionConflict) {
		t.Fatalf("陈旧删除 error = %v, want ErrRuntimeVersionConflict", err)
	}
	current, err := service.GetAgent(ctx, created.AgentID)
	if err != nil {
		t.Fatalf("陈旧删除后 Agent 不应消失: %v", err)
	}
	if current.RuntimeVersion != updated.RuntimeVersion || current.Name != nextName {
		t.Fatalf("陈旧删除改变了 Agent: %+v", current)
	}
	if content, readErr := os.ReadFile(markerPath); readErr != nil || string(content) != "keep" {
		t.Fatalf("陈旧删除先破坏了 workspace: content=%q err=%v", content, readErr)
	}

	if err = service.DeleteAgentAtVersion(
		ctx,
		created.AgentID,
		updated.RuntimeVersion,
	); err != nil {
		t.Fatalf("当前版本删除失败: %v", err)
	}
	if _, statErr := os.Stat(created.WorkspacePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("成功删除后 workspace 仍存在: %v", statErr)
	}
}

func TestServiceAllowsSelfNameValidationAndCaseOnlyRename(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "sam"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "Sam", created.AgentID)
	if err != nil {
		t.Fatalf("大小写改名校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("同一 agent 只改大小写时名称应该可用: %+v", validation)
	}

	nextName := "Sam"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("大小写改名失败: %v", err)
	}
	if updated.Name != "Sam" {
		t.Fatalf("大小写改名未生效: %+v", updated)
	}
}

func TestServiceAllowsDuplicateAndSlugCollidingAgentNames(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db := newAgentTestService(t, cfg)

	ctx := context.Background()
	first, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a b"})
	if err != nil {
		t.Fatalf("创建基准 agent 失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "a_b", "")
	if err != nil {
		t.Fatalf("校验 slug 冲突名称失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("名称派生 slug 冲突不应阻断创建: %+v", validation)
	}

	second, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a_b"})
	if err != nil {
		t.Fatalf("创建名称派生 slug 冲突 agent 不应失败: %v", err)
	}
	third, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "a b"})
	if err != nil {
		t.Fatalf("重复展示名不应阻断创建: %v", err)
	}
	if first.AgentID == second.AgentID || first.AgentID == third.AgentID || second.AgentID == third.AgentID {
		t.Fatalf("重复展示名应创建独立 agent_id: first=%s second=%s third=%s", first.AgentID, second.AgentID, third.AgentID)
	}
	if slug := agentSlug(t, db, first.AgentID); slug != first.AgentID {
		t.Fatalf("新建 agent slug 应绑定 agent_id: got=%s want=%s", slug, first.AgentID)
	}
	if slug := agentSlug(t, db, second.AgentID); slug != second.AgentID {
		t.Fatalf("新建 agent slug 应绑定 agent_id: got=%s want=%s", slug, second.AgentID)
	}

	nextName := "a_b"
	updated, err := service.UpdateAgent(ctx, first.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("改成其他 agent 的展示名不应失败: %v", err)
	}
	if updated.Name != "a_b" {
		t.Fatalf("展示名改名未生效: %+v", updated)
	}
	if slug := agentSlug(t, db, first.AgentID); slug != first.AgentID {
		t.Fatalf("改名不应改变 agent slug: got=%s want=%s", slug, first.AgentID)
	}
}

func TestServiceHardDeletesAgentAndAllowsNameReuse(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db := newAgentTestService(t, cfg)

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "可重建助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO im_channel_configs (owner_user_id, channel_type, agent_id, status, config_json)
VALUES (?, 'telegram', ?, 'configured', '{}');
INSERT INTO im_channel_accounts (owner_user_id, channel_type, account_id, status, config_json)
VALUES (?, 'telegram', 'account-a', 'connected', '{}');
INSERT INTO im_pairings (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, agent_id, status, source
) VALUES ('pair-agent-delete', ?, 'telegram', 'dm', 'chat-a', ?, 'active', 'manual');`,
		created.OwnerUserID,
		created.AgentID,
		created.OwnerUserID,
		created.OwnerUserID,
		created.AgentID,
	); err != nil {
		t.Fatalf("准备 Agent Channel 级联数据失败: %v", err)
	}
	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("删除 agent 失败: %v", err)
	}

	assertNoRowsForAgent(t, db, "agents", "id", created.AgentID)
	assertNoRowsForAgent(t, db, "profiles", "agent_id", created.AgentID)
	assertNoRowsForAgent(t, db, "runtimes", "agent_id", created.AgentID)
	assertNoRowsForAgent(t, db, "im_channel_configs", "agent_id", created.AgentID)
	assertNoRowsForAgent(t, db, "im_pairings", "agent_id", created.AgentID)
	assertNoRowsForAgent(t, db, "im_channel_accounts", "account_id", "account-a")
	var channelVersion int64
	if err = db.QueryRow(
		"SELECT version FROM channel_control_versions WHERE owner_user_id = ?",
		created.OwnerUserID,
	).Scan(&channelVersion); err != nil || channelVersion != 2 {
		t.Fatalf("Agent 删除应在同一事务推进 Channel version: version=%d err=%v", channelVersion, err)
	}

	if _, err = service.GetAgent(ctx, created.AgentID); !errors.Is(err, agentpkg.ErrAgentNotFound) {
		t.Fatalf("硬删除后读取 agent 应返回不存在: %v", err)
	}

	recreated, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "可重建助手"})
	if err != nil {
		t.Fatalf("删除后应允许复用名称: %v", err)
	}
	if recreated.AgentID == created.AgentID {
		t.Fatalf("复用名称应创建新的 agent_id: old=%s new=%s", created.AgentID, recreated.AgentID)
	}
}

func TestServiceDeleteAgentIgnoresWorkspaceMarkerCleanupFailure(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, db := newAgentTestService(t, cfg)
	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "可清理助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	cleaner := &failingWorkspaceStateCleaner{}
	service.SetWorkspaceManager(cleaner)

	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("可重建 marker 清理失败不应阻断 Agent 删除: %v", err)
	}
	if cleaner.removeCalls != 1 {
		t.Fatalf("workspace marker 清理次数 = %d, want 1", cleaner.removeCalls)
	}
	assertNoRowsForAgent(t, db, "agents", "id", created.AgentID)
}

func TestServiceUsesAgentIDWorkspacePathAndRenameKeepsWorkspace(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "chatbuddy"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if filepath.Base(created.WorkspacePath) != agentpkg.BuildWorkspaceDirName(created.AgentID) {
		t.Fatalf("workspace 目录应使用 agent_id: got=%s agent_id=%s", created.WorkspacePath, created.AgentID)
	}
	workspaceMarker := filepath.Join(created.WorkspacePath, "marker.txt")
	if err = os.WriteFile(workspaceMarker, []byte("ok"), 0o644); err != nil {
		t.Fatalf("写入 workspace 标记失败: %v", err)
	}
	agentsFile := filepath.Join(created.WorkspacePath, "AGENTS.md")
	customAgentsContent := "# AGENTS.md\n\n用户自定义规则\n"
	if err = os.WriteFile(agentsFile, []byte(customAgentsContent), 0o644); err != nil {
		t.Fatalf("写入 AGENTS.md 失败: %v", err)
	}
	if err = os.MkdirAll(filepath.Join(cfg.WorkspacePath, "chat"), 0o755); err != nil {
		t.Fatalf("创建冲突候选目录失败: %v", err)
	}

	validation, err := service.ValidateName(ctx, "chat", created.AgentID)
	if err != nil {
		t.Fatalf("编辑态名称校验失败: %v", err)
	}
	if !validation.IsValid || !validation.IsAvailable {
		t.Fatalf("agent_id 目录模式不应被同名目录阻断: %+v", validation)
	}

	nextName := "chat"
	updated, err := service.UpdateAgent(ctx, created.AgentID, protocol.UpdateRequest{Name: &nextName})
	if err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	if updated.Name != "chat" {
		t.Fatalf("改名未生效: %+v", updated)
	}
	if updated.WorkspacePath != created.WorkspacePath {
		t.Fatalf("改名不应移动 workspace_path: got=%s want=%s", updated.WorkspacePath, created.WorkspacePath)
	}
	if _, err = os.Stat(filepath.Join(updated.WorkspacePath, "marker.txt")); err != nil {
		t.Fatalf("workspace 内容应保留在原目录: %v", err)
	}
	agentsContent, err := os.ReadFile(agentsFile)
	if err != nil {
		t.Fatalf("读取 AGENTS.md 失败: %v", err)
	}
	if string(agentsContent) != customAgentsContent {
		t.Fatalf("改名不应重写 AGENTS.md 系统身份字段: %s", agentsContent)
	}
}

func TestDeleteAgentRemovesTranscriptProject(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)

	service, _ := newAgentTestService(t, cfg)
	goalCleaner := &fakeAgentGoalCleaner{}
	service.SetGoalCleaner(goalCleaner)

	ctx := context.Background()
	created, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: "删除助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	projectDir := agentTranscriptProjectDir(created.WorkspacePath)
	if err = os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 项目目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, "delete-session.jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript 文件失败: %v", err)
	}
	if err = json.NewEncoder(file).Encode(map[string]any{
		"type":      "user",
		"uuid":      "delete-user-1",
		"sessionId": "delete-session",
		"message": map[string]any{
			"role":    "user",
			"content": "你好",
		},
	}); err != nil {
		_ = file.Close()
		t.Fatalf("写入 transcript 文件失败: %v", err)
	}
	_ = file.Close()

	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("删除 agent 失败: %v", err)
	}
	if _, err = os.Stat(projectDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 agent 后 transcript 项目目录仍残留: %v", err)
	}
	if len(goalCleaner.agentIDs) != 1 || goalCleaner.agentIDs[0] != created.AgentID {
		t.Fatalf("goal cleaner agent IDs = %#v, want deleted agent", goalCleaner.agentIDs)
	}
}

type fakeAgentGoalCleaner struct {
	agentIDs []string
}

type failingWorkspaceStateCleaner struct {
	removeCalls int
}

type recordingWorkspaceManager struct {
	initialized   []protocol.Agent
	initializeErr error
}

func (r *recordingWorkspaceManager) InitializeAgentWorkspace(
	_ context.Context,
	agentValue protocol.Agent,
) error {
	r.initialized = append(r.initialized, agentValue)
	return r.initializeErr
}

func (*recordingWorkspaceManager) RemoveAgentWorkspaceState(
	context.Context,
	protocol.Agent,
) error {
	return nil
}

func (*failingWorkspaceStateCleaner) InitializeAgentWorkspace(
	context.Context,
	protocol.Agent,
) error {
	return nil
}

func (f *failingWorkspaceStateCleaner) RemoveAgentWorkspaceState(
	context.Context,
	protocol.Agent,
) error {
	f.removeCalls++
	return errors.New("marker cleanup failed")
}

func (f *fakeAgentGoalCleaner) DeleteGoalsForAgent(_ context.Context, agentID string) (int, error) {
	f.agentIDs = append(f.agentIDs, agentID)
	return 1, nil
}

func newTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18010,
		ProjectName:    "nexus-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

var agentTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func agentTranscriptProjectDir(workspacePath string) string {
	return filepath.Join(
		appfs.UserRuntimeRoot(authctx.SystemUserID),
		"projects",
		sanitizeAgentTranscriptPath(canonicalizeAgentTranscriptPath(workspacePath)),
	)
}

func assertRuntimeEmotionStateFile(t *testing.T, workspacePath string) {
	t.Helper()
	statePath := filepath.Join(workspacePath, ".agents", "emotion.json")
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("emotion state 未初始化: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("emotion state 应为文件: %s", statePath)
	}
	if info.Size() != 0 {
		t.Fatalf("emotion state 初始文件应为空: size=%d", info.Size())
	}
}

func assertNoRowsForAgent(t *testing.T, db *sql.DB, table string, column string, value string) {
	t.Helper()

	var count int
	query := "SELECT COUNT(1) FROM " + table + " WHERE " + column + " = ?"
	if err := db.QueryRow(query, value).Scan(&count); err != nil {
		t.Fatalf("查询 %s.%s 失败: %v", table, column, err)
	}
	if count != 0 {
		t.Fatalf("删除 agent 后 %s 仍有残留: %d", table, count)
	}
}

func agentSlug(t *testing.T, db *sql.DB, agentID string) string {
	t.Helper()

	var slug string
	if err := db.QueryRow(`SELECT slug FROM agents WHERE id = ?`, agentID).Scan(&slug); err != nil {
		t.Fatalf("查询 agent slug 失败: %v", err)
	}
	return slug
}

func canonicalizeAgentTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if absolutePath, err := filepath.Abs(path); err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeAgentTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := agentTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + agentTranscriptHash(path)
}

func agentTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func migrateSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, testMigrationDir(t))
}

func newAgentTestService(t *testing.T, cfg config.Config) (*agentpkg.Service, *sql.DB) {
	t.Helper()
	service, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 service 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭 Agent 测试数据库失败: %v", err)
		}
	})
	return service, db
}

func testMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
