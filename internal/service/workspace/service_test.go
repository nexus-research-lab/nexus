package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServiceManagesWorkspaceFiles(t *testing.T) {
	t.Setenv(nexusctlCommandPathEnvName, "")
	t.Setenv(nexuscfgCommandPathEnvName, "")
	t.Setenv(nexusCommandPathEnvName, "")
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("初始化测试平台 Skill 失败: %v", err)
	}

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "工作区测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	files, err := workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出 workspace 文件失败: %v", err)
	}
	if containsWorkspacePath(files, "AGENTS.md") {
		t.Fatalf("Agent 身份文件不应出现在 workspace 文件树: %+v", files)
	}
	agentsContent, err := workspaceService.GetFile(ctx, agentValue.AgentID, "AGENTS.md")
	if err != nil {
		t.Fatalf("Agent 身份文件仍应可由身份面板读取: %v", err)
	}
	if !strings.Contains(agentsContent.Content, "## Role") {
		t.Fatalf("Agent 身份文件内容不正确: %q", agentsContent.Content)
	}
	if strings.Contains(agentsContent.Content, "# AGENTS.md") {
		t.Fatalf("Agent 身份文件不应在模板正文中注入文件名: %q", agentsContent.Content)
	}
	for _, expectedPath := range []string{"USER.md", "SOUL.md", "TOOLS.md"} {
		if !containsWorkspacePath(files, expectedPath) {
			t.Fatalf("初始化模板未生成 %s: %+v", expectedPath, files)
		}
	}
	for _, unexpectedPath := range []string{"MEMORY.md", "RUNBOOK.md"} {
		if containsWorkspacePath(files, unexpectedPath) {
			t.Fatalf("普通 agent 不应默认生成 %s: %+v", unexpectedPath, files)
		}
	}
	attachmentPath := filepath.Join(agentValue.WorkspacePath, "tmp", "attachments", "demo", "input.md")
	if err = os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("创建附件目录失败: %v", err)
	}
	if err = os.WriteFile(attachmentPath, []byte("# 附件"), 0o644); err != nil {
		t.Fatalf("写入附件失败: %v", err)
	}
	files, err = workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出带附件 workspace 文件失败: %v", err)
	}
	if !containsWorkspacePath(files, "tmp/attachments/demo/input.md") {
		t.Fatalf("文件树应展示临时附件目录: %+v", files)
	}
	attachmentContent, err := workspaceService.GetFile(ctx, agentValue.AgentID, "tmp/attachments/demo/input.md")
	if err != nil {
		t.Fatalf("附件路径应允许消息预览读取: %v", err)
	}
	if attachmentContent.Content != "# 附件" {
		t.Fatalf("附件内容读取不正确: %+v", attachmentContent)
	}
	uploadedAttachment, err := workspaceService.UploadFile(ctx, agentValue.AgentID, "upload.txt", "tmp/attachments/upload-batch/", strings.NewReader("upload attachment"))
	if err != nil {
		t.Fatalf("上传附件到 tmp/attachments 失败: %v", err)
	}
	if uploadedAttachment.Path != "tmp/attachments/upload-batch/upload.txt" {
		t.Fatalf("附件上传路径不正确: %+v", uploadedAttachment)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, "tmp", "attachments", "upload-batch", "upload.txt")); err != nil {
		t.Fatalf("附件未落盘到 tmp/attachments: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "imagegen", "SKILL.md")); err != nil {
		t.Fatalf("平台全局 imagegen skill 未同步: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "imagegen", "SKILL.md")); err != nil {
		t.Fatalf("Claude 兼容 imagegen skill 未同步: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "visualize", "SKILL.md")); err != nil {
		t.Fatalf("平台全局 visualize skill 未同步: %v", err)
	}
	if _, err = os.Stat(filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "automation", "SKILL.md")); err != nil {
		t.Fatalf("平台全局 automation skill 未同步: %v", err)
	}
	if !slices.Contains(agentValue.Options.SkillIDs, "imagegen") ||
		!slices.Contains(agentValue.Options.SkillIDs, "visualize") ||
		!slices.Contains(agentValue.Options.SkillIDs, "automation") ||
		!slices.Contains(agentValue.Options.SkillIDs, "goal-manager") ||
		!slices.Contains(agentValue.Options.SkillIDs, "execution-orchestrator") {
		t.Fatalf("Agent 应只记录平台 Skill ID: %#v", agentValue.Options.SkillIDs)
	}
	sharedBinDir := filepath.Join(os.Getenv("NEXUS_CONFIG_DIR"), "app", ".agents", "bin")
	nexusctlShim := filepath.Join(sharedBinDir, "nexusctl")
	if info, statErr := os.Stat(nexusctlShim); statErr != nil {
		t.Fatalf("共享 nexusctl shim 未生成: %v", statErr)
	} else if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Fatalf("nexusctl shim 应可执行: %s", nexusctlShim)
	}
	shimPayload, err := os.ReadFile(nexusctlShim)
	if err != nil {
		t.Fatalf("读取 nexusctl shim 失败: %v", err)
	}
	if !strings.Contains(string(shimPayload), "NEXUSCTL_WORKSPACE_PATH") {
		t.Fatalf("nexusctl shim 应保留调用方 workspace 路径: %s", shimPayload)
	}
	if !strings.Contains(string(shimPayload), "go run ./cmd/nexusctl") {
		t.Fatalf("开发环境 nexusctl shim 应固定到源码入口: %s", shimPayload)
	}
	for _, unexpected := range []string{"$PROJECT_ROOT/bin/nexusctl", "$PROJECT_ROOT/nexusctl"} {
		if strings.Contains(string(shimPayload), unexpected) {
			t.Fatalf("nexusctl shim 不应再运行期多路径 fallback: %s", shimPayload)
		}
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "bin", "nexusctl")); !os.IsNotExist(err) {
		t.Fatalf("agent workspace 不应生成独立 nexusctl shim: %v", err)
	}
	nexuscfgShim := filepath.Join(sharedBinDir, "nexuscfg")
	if info, statErr := os.Stat(nexuscfgShim); statErr != nil {
		t.Fatalf("共享 nexuscfg shim 未生成: %v", statErr)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("nexuscfg shim 应可执行: %s", nexuscfgShim)
	}
	nexuscfgPayload, err := os.ReadFile(nexuscfgShim)
	if err != nil {
		t.Fatalf("读取 nexuscfg shim 失败: %v", err)
	}
	if !strings.Contains(string(nexuscfgPayload), "go run ./cmd/nexuscfg") {
		t.Fatalf("开发环境 nexuscfg shim 应固定到源码入口: %s", nexuscfgPayload)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "bin", "nexuscfg")); !os.IsNotExist(err) {
		t.Fatalf("agent workspace 不应生成独立 nexuscfg shim: %v", err)
	}
	nexusShim := filepath.Join(sharedBinDir, "nexus")
	if info, statErr := os.Stat(nexusShim); statErr != nil {
		t.Fatalf("共享 nexus shim 未生成: %v", statErr)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("nexus shim 应可执行: %s", nexusShim)
	}
	nexusPayload, err := os.ReadFile(nexusShim)
	if err != nil {
		t.Fatalf("读取 nexus shim 失败: %v", err)
	}
	if !strings.Contains(string(nexusPayload), protocol.NexusCommandHostEntrypointArgument) ||
		strings.Contains(string(nexusPayload), "go run ./cmd/nexus") {
		t.Fatalf("nexus shim 应复用稳定宿主入口，不能在 Agent round 内编译源码: %s", nexusPayload)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "bin", "nexus")); !os.IsNotExist(err) {
		t.Fatalf("agent workspace 不应生成独立 nexus shim: %v", err)
	}
	staleImagegenScript := filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "imagegen", "scripts", "image_gen.py")
	if err = os.MkdirAll(filepath.Dir(staleImagegenScript), 0o755); err != nil {
		t.Fatalf("创建 stale imagegen 目录失败: %v", err)
	}
	if err = os.WriteFile(staleImagegenScript, []byte("stale"), 0o644); err != nil {
		t.Fatalf("写入 stale imagegen 脚本失败: %v", err)
	}
	retiredScheduledSkillDirs := []string{
		filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "scheduled-task-manager"),
		filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "scheduled-task-manager"),
	}
	if err = EnsureInitialized(agentValue.AgentID, agentValue.Name, agentValue.WorkspacePath, agentValue.IsMain, agentValue.CreatedAt); err != nil {
		t.Fatalf("重新初始化 workspace 失败: %v", err)
	}
	if _, err = os.Stat(staleImagegenScript); !os.IsNotExist(err) {
		t.Fatalf("系统托管 skill 同步后应删除已移除脚本: %v", err)
	}
	for _, skillDir := range retiredScheduledSkillDirs {
		if _, statErr := os.Lstat(skillDir); !os.IsNotExist(statErr) {
			t.Fatalf("workspace 初始化后仍保留已退役定时任务 skill %s: %v", skillDir, statErr)
		}
	}
	platformAgentSkills := filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills")
	managedSkillContracts := map[string][]string{
		filepath.Join("automation", "SKILL.md"): {
			"NEXUS_COMMAND_PATH",
			"automation contract",
			"inspect → plan → apply",
			"原生真人确认",
			"后台 scheduled run 只有查询权限",
			"IM `/y`、`/a`、`/d`",
			"references/operations.md",
		},
		filepath.Join("automation", "references", "operations.md"): {
			"Automation CLI 操作",
			"retry_delivery",
			"cross_agent_allowed",
			"current_revision",
		},
		filepath.Join("visualize", "SKILL.md"): {
			"在 Nexus 对话中生成交互式图表",
			"Call `show_widget`",
			"Network access and external resources are allowed without a domain allowlist",
			"Never call `.addEventListener` directly",
			"Missing widget element",
			"--nexus-chart-1",
		},
		filepath.Join("goal-manager", "SKILL.md"): {
			"--json goal contract",
			"input_staging.path",
			"references/create-and-retarget.md",
			"execution-orchestrator",
			"token_budget",
		},
		filepath.Join("goal-manager", "references", "create-and-retarget.md"): {
			"promote_execution_to_goal",
			"信息足够前禁止调用 `create_goal`",
			"保留同一 Goal 身份和累计用量",
		},
		filepath.Join("goal-manager", "references", "complete-and-block.md"): {
			"audit_objective_alignment",
			"最终回复必须脱离过程消息",
			"至少连续三个 Goal turns",
		},
		filepath.Join("goal-manager", "references", "room-goals.md"): {
			"Lead 身份",
			"Work Item + Assignment",
			"可见协作审计",
			"不是完成门槛",
		},
		filepath.Join("execution-orchestrator", "SKILL.md"): {
			"Goal 决定持续追求什么",
			"--json execution contract",
			"input_staging.path",
			"最小选择表",
			"references/structure-selection.md",
			"substantial execution 前评估任务是否原子",
			"只加入价值高于协调成本的结构",
			"不因 handoff 要求用户发送“继续”",
		},
		filepath.Join("execution-orchestrator", "references", "structure-selection.md"): {
			"独立判断信号",
			"用例只是校验",
			"加载 `goal-manager`",
		},
		filepath.Join("execution-orchestrator", "references", "graph-control.md"): {
			"两层图",
			"nexus_plan: 1",
			"document_contract",
			"Skill 不复制完整字段表",
			"自审折叠在同一 Agent 节点",
			"Goal 生命周期不是使用 Loop 的前提",
		},
		filepath.Join("execution-orchestrator", "references", "communication-and-continuity.md"): {
			"四个平面",
			"command 可以记录交付状态",
			"用户有没有说“协作”",
			"不要要求用户发送“继续”",
		},
	}
	for relativePath, expectedValues := range managedSkillContracts {
		payload, readErr := os.ReadFile(filepath.Join(platformAgentSkills, relativePath))
		if readErr != nil {
			t.Fatalf("读取系统托管 Skill 文件 %s 失败: %v", relativePath, readErr)
		}
		for _, expected := range expectedValues {
			if !strings.Contains(string(payload), expected) {
				t.Fatalf("系统托管 Skill 文件 %s 缺少 %q", relativePath, expected)
			}
		}
	}

	updated, err := workspaceService.UpdateFile(ctx, agentValue.AgentID, "notes/todo.md", "hello workspace")
	if err != nil {
		t.Fatalf("更新文件失败: %v", err)
	}
	if updated.Path != "notes/todo.md" {
		t.Fatalf("文件路径不正确: %+v", updated)
	}

	readBack, err := workspaceService.GetFile(ctx, agentValue.AgentID, "notes/todo.md")
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if readBack.Content != "hello workspace" {
		t.Fatalf("文件内容不匹配: %+v", readBack)
	}

	if _, err = workspaceService.CreateEntry(ctx, agentValue.AgentID, "docs", "directory", ""); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	renamed, err := workspaceService.RenameEntry(ctx, agentValue.AgentID, "notes/todo.md", "docs/todo.md")
	if err != nil {
		t.Fatalf("重命名文件失败: %v", err)
	}
	if renamed.NewPath != "docs/todo.md" {
		t.Fatalf("重命名结果不正确: %+v", renamed)
	}

	if _, err = workspaceService.DeleteEntry(ctx, agentValue.AgentID, "docs/todo.md"); err != nil {
		t.Fatalf("删除文件失败: %v", err)
	}
	if _, err = workspaceService.GetFile(ctx, agentValue.AgentID, "docs/todo.md"); err == nil {
		t.Fatal("删除后仍能读取文件")
	}

	if _, err = workspaceService.UpdateFile(ctx, agentValue.AgentID, ".agents/forbidden.txt", "x"); err == nil {
		t.Fatal("不应允许直接写入内部运行时目录")
	}
	if _, err = workspaceService.UpdateFile(ctx, agentValue.AgentID, "nested/.git/config", "x"); err == nil {
		t.Fatal("不应允许写入嵌套仓库内部目录")
	}
}

func TestServiceInitializesDefaultMainWorkspaceDuringBootstrap(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)

	mainAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("创建默认主 Agent 失败: %v", err)
	}
	root, err := workspaceService.openAgentWorkspace(mainAgent, false)
	if err != nil {
		t.Fatalf("打开默认主 Agent workspace 失败: %v", err)
	}
	defer root.Close()
	if !workspaceManagedStateReady(root, true) {
		t.Fatal("默认主 Agent 落库前应完成完整 workspace 初始化")
	}

	stateRoot, marker, err := openWorkspaceInitializationState(*mainAgent)
	if err != nil {
		t.Fatalf("打开默认主 Agent 初始化状态失败: %v", err)
	}
	defer stateRoot.Close()
	payload, err := stateRoot.ReadFile(marker)
	if err != nil {
		t.Fatalf("默认主 Agent 缺少初始化 marker: %v", err)
	}
	if strings.TrimSpace(string(payload)) != workspaceInitializationVersion(*mainAgent) {
		t.Fatalf("默认主 Agent 初始化 marker 不匹配: %q", payload)
	}
}

func TestListFilesDoesNotInitializeWorkspace(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "只读文件树助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	if err = os.Remove(filepath.Join(agentValue.WorkspacePath, "USER.md")); err != nil {
		t.Fatalf("删除初始化产物 USER.md 失败: %v", err)
	}

	files, err := workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出 workspace 文件失败: %v", err)
	}
	if containsWorkspacePath(files, "USER.md") {
		t.Fatalf("文件列表不应补写缺失模板: %+v", files)
	}
	if _, statErr := os.Stat(filepath.Join(agentValue.WorkspacePath, "USER.md")); !os.IsNotExist(statErr) {
		t.Fatalf("文件列表产生了初始化副作用 USER.md: %v", statErr)
	}
}
