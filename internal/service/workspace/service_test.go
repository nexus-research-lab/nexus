package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestServiceManagesWorkspaceFiles(t *testing.T) {
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

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "工作区测试助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	files, err := workspaceService.ListFiles(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("列出 workspace 文件失败: %v", err)
	}
	if !containsWorkspacePath(files, "AGENTS.md") {
		t.Fatalf("初始化模板未生成 AGENTS.md: %+v", files)
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
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "imagegen", "SKILL.md")); err != nil {
		t.Fatalf("系统托管 imagegen skill 未部署: %v", err)
	}
	sharedBinDir := filepath.Join(os.Getenv("NEXUS_CONFIG_DIR"), ".agents", "bin")
	nexusctlShim := filepath.Join(sharedBinDir, "nexusctl")
	if info, statErr := os.Stat(nexusctlShim); statErr != nil {
		t.Fatalf("共享 nexusctl shim 未生成: %v", statErr)
	} else if info.Mode()&0o111 == 0 {
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
	nexusctlCmdShim := filepath.Join(sharedBinDir, "nexusctl.cmd")
	cmdPayload, err := os.ReadFile(nexusctlCmdShim)
	if err != nil {
		t.Fatalf("Windows nexusctl shim 未生成: %v", err)
	}
	if !strings.Contains(string(cmdPayload), "go run ./cmd/nexusctl") {
		t.Fatalf("Windows nexusctl shim 应固定到源码入口: %s", cmdPayload)
	}
	if _, err = os.Stat(filepath.Join(agentValue.WorkspacePath, ".agents", "bin", "nexusctl")); !os.IsNotExist(err) {
		t.Fatalf("agent workspace 不应生成独立 nexusctl shim: %v", err)
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
	goalSkillPath := filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "goal-manager", "SKILL.md")
	if _, err = os.Stat(goalSkillPath); err != nil {
		t.Fatalf("系统托管 goal-manager skill 未部署: %v", err)
	}
	goalSkill, err := os.ReadFile(goalSkillPath)
	if err != nil {
		t.Fatalf("读取 goal-manager skill 失败: %v", err)
	}
	for _, expected := range []string{
		"nexus_goal",
		"mcp__nexus_goal__get_goal",
		"mcp__nexus_goal__create_goal",
		"mcp__nexus_goal__retarget_goal",
		"mcp__nexus_goal__update_goal",
		"create_goal",
		"get_goal",
		"retarget_goal",
		"update_goal",
		"Skill 只负责加载这份使用说明",
		"不要用 /goal 文本命令",
	} {
		if !strings.Contains(string(goalSkill), expected) {
			t.Fatalf("goal-manager skill 缺少 %q", expected)
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
