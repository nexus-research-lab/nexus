package workspace

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestServiceManagesWorkspaceFiles(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
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
	for _, expectedPath := range []string{"USER.md", "MEMORY.md", "SOUL.md", "TOOLS.md"} {
		if !containsWorkspacePath(files, expectedPath) {
			t.Fatalf("初始化模板未生成 %s: %+v", expectedPath, files)
		}
	}
	for _, unexpectedPath := range []string{"RUNBOOK.md"} {
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
	nexusctlCmdShim := filepath.Join(sharedBinDir, "nexusctl.cmd")
	cmdPayload, err := os.ReadFile(nexusctlCmdShim)
	if err != nil {
		t.Fatalf("Windows nexusctl shim 未生成: %v", err)
	}
	if !strings.Contains(string(cmdPayload), "nexusctl.exe") {
		t.Fatalf("Windows nexusctl shim 未查找 exe: %s", cmdPayload)
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
	if err = EnsureInitialized(agentValue.AgentID, agentValue.Name, agentValue.WorkspacePath, agentValue.IsMain, agentValue.CreatedAt); err != nil {
		t.Fatalf("重新初始化 workspace 失败: %v", err)
	}
	if _, err = os.Stat(staleImagegenScript); !os.IsNotExist(err) {
		t.Fatalf("系统托管 skill 同步后应删除已移除脚本: %v", err)
	}
	scheduledTaskSkillPath := filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "scheduled-task-manager", "SKILL.md")
	if _, err = os.Stat(scheduledTaskSkillPath); err != nil {
		t.Fatalf("系统托管 scheduled-task-manager skill 未部署: %v", err)
	}
	scheduledTaskSkill, err := os.ReadFile(scheduledTaskSkillPath)
	if err != nil {
		t.Fatalf("读取 scheduled-task-manager skill 失败: %v", err)
	}
	for _, expected := range []string{
		"get_scheduled_task_daily_report",
		"get_scheduled_task_status",
		"retry_scheduled_task_delivery",
		"reply_mode\": \"channel\"",
		"reply_mode\": \"agent\"",
		"larksuite/lark-openapi-mcp",
	} {
		if !strings.Contains(string(scheduledTaskSkill), expected) {
			t.Fatalf("scheduled-task-manager skill 缺少 %q", expected)
		}
	}
	claudeSkillLink := filepath.Join(agentValue.WorkspacePath, ".claude", "skills", "scheduled-task-manager")
	if info, statErr := os.Lstat(claudeSkillLink); statErr != nil {
		t.Fatalf("scheduled-task-manager skill 的 Claude 链接未生成: %v", statErr)
	} else if info.Mode()&os.ModeSymlink == 0 {
		if !info.IsDir() {
			t.Fatalf("scheduled-task-manager skill 的 Claude 入口应为符号链接或镜像目录: %s", claudeSkillLink)
		}
		if _, err = os.Stat(filepath.Join(claudeSkillLink, "SKILL.md")); err != nil {
			t.Fatalf("scheduled-task-manager skill 的 Claude 镜像目录缺少 SKILL.md: %v", err)
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

func TestWorkspaceHiddenEntryMatchesNestedHeavyDirs(t *testing.T) {
	testCases := []string{
		".git/config",
		"repo/.git/config",
		"repo/.claude/settings.json",
		"repo/node_modules/pkg/index.js",
		"repo/web/node_modules/pkg/index.js",
		"repo/web/.next/server/app.js",
		"repo/web/dist/assets/main.js",
		"repo/coverage/index.html",
		"repo/__pycache__/cache.pyc",
		"repo/.DS_Store",
	}
	for _, testCase := range testCases {
		if !shouldHideWorkspaceEntry(testCase) {
			t.Fatalf("应隐藏 workspace 重目录: %s", testCase)
		}
	}

	visibleCases := []string{
		"repo/internal/service/workspace/service.go",
		"repo/web/src/main.tsx",
		"repo/docs/spec.md",
		"tmp/attachments/demo/input.md",
	}
	for _, testCase := range visibleCases {
		if shouldHideWorkspaceEntry(testCase) {
			t.Fatalf("不应隐藏普通 workspace 文件: %s", testCase)
		}
	}
}

func TestEnsureInitializedWritesPromptLayerTemplates(t *testing.T) {
	root := t.TempDir()
	if err := EnsureInitialized("agent-1", "Planner", root, false, time.Now()); err != nil {
		t.Fatalf("初始化普通 agent workspace 失败: %v", err)
	}
	for fileName, expected := range map[string]string{
		"AGENTS.md": "Follow the injected Agent Identity, Agent Profile",
		"USER.md":   "replace this entire file with a configured profile",
		"MEMORY.md": "Long-Term Memory",
		"SOUL.md":   "## Emotion",
		"TOOLS.md":  "## Tool Notes",
	} {
		assertWorkspaceFileContains(t, root, fileName, expected)
	}
	for _, fileName := range []string{"RUNBOOK.md"} {
		if _, err := os.Stat(filepath.Join(root, fileName)); !os.IsNotExist(err) {
			t.Fatalf("普通 agent 不应默认生成 %s: %v", fileName, err)
		}
	}
	defaultAgentsContent, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("读取普通 agent AGENTS.md 失败: %v", err)
	}
	if strings.Contains(string(defaultAgentsContent), "You are Nexus, a personal workspace agent") {
		t.Fatalf("普通 agent 模板不应把身份写死成 Nexus: %s", defaultAgentsContent)
	}
	for _, unexpected := range []string{
		"main Nexus agent organizes collaboration",
		"nexus_automation",
		"scheduled-task-manager",
		"nexusctl memory",
		"Room titles must be specific",
	} {
		if strings.Contains(string(defaultAgentsContent), unexpected) {
			t.Fatalf("普通 agent 模板不应包含 main/tool 固定职责 %q: %s", unexpected, defaultAgentsContent)
		}
	}
	if strings.Contains(string(defaultAgentsContent), "Identity:") || strings.Contains(string(defaultAgentsContent), "WORKING DIRECTORY:") {
		t.Fatalf("普通 agent 模板不应暴露系统身份字段: %s", defaultAgentsContent)
	}

	mainRoot := t.TempDir()
	if err := EnsureInitialized("nexus", "Nexus", mainRoot, true, time.Now()); err != nil {
		t.Fatalf("初始化 main agent workspace 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainRoot, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("main agent 不应默认生成 AGENTS.md 暴露内部提示词: %v", err)
	}
	assertWorkspaceFileContains(t, mainRoot, "USER.md", "setup_status: unconfigured")
	assertWorkspaceFileContains(t, mainRoot, "USER.md", "Replace this template instead of appending below it")
	assertWorkspaceFileContains(t, mainRoot, "MEMORY.md", "Long-Term Memory")
	for _, fileName := range []string{"SOUL.md", "TOOLS.md", "RUNBOOK.md"} {
		if _, err := os.Stat(filepath.Join(mainRoot, fileName)); !os.IsNotExist(err) {
			t.Fatalf("main agent 不应默认生成 %s: %v", fileName, err)
		}
	}
}

func TestEnsureInitializedSerializesConcurrentSkillDeployment(t *testing.T) {
	root := t.TempDir()
	createdAt := time.Now()
	const workerCount = 16

	var wg sync.WaitGroup
	errs := make(chan error, workerCount)
	for index := 0; index < workerCount; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- EnsureInitialized("agent-1", "Planner", root, false, createdAt)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("并发初始化 workspace 不应互相删除托管 skill: %v", err)
		}
	}
	for _, skillName := range managedSkillNames(false) {
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", skillName, "SKILL.md")); err != nil {
			t.Fatalf("并发初始化后托管 skill 缺失 %s: %v", skillName, err)
		}
	}
}

func TestEnsureInitializedRepairsStaleScheduleWakeupGuidance(t *testing.T) {
	cases := []struct {
		name        string
		isMainAgent bool
		heading     string
	}{
		{name: "default agent", heading: "## 定时任务"},
		{name: "main agent", isMainAgent: true, heading: "## 定时任务路由"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			stale := "# AGENTS.md\n\n## Agent Profile\n\n用户自定义内容\n\n" + tc.heading + "\n\n" +
				"- **ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒**\n" +
				"  仅在**全部**满足时使用：一次性、延迟 < 30 分钟、只活在当前会话里、丢了不影响用户目标。\n\n" +
				"## Custom\n\n保留我\n"
			if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(stale), 0o644); err != nil {
				t.Fatalf("写入旧 AGENTS.md 失败: %v", err)
			}

			err := EnsureInitialized("agent-1", "测试助手", root, tc.isMainAgent, time.Now())
			if err != nil {
				t.Fatalf("初始化 workspace 失败: %v", err)
			}

			repaired, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
			if err != nil {
				t.Fatalf("读取修复后 AGENTS.md 失败: %v", err)
			}
			got := string(repaired)
			assertNoStaleScheduleWakeupGuidance(t, got)
			if !strings.Contains(got, "用户自定义内容") || !strings.Contains(got, "## Custom\n\n保留我") {
				t.Fatalf("修复不应覆盖用户自定义内容: %s", got)
			}
		})
	}
}

func TestUploadFileToRootReusesIdenticalTargetByMD5(t *testing.T) {
	root := t.TempDir()

	first, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	second, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("重复上传失败: %v", err)
	}
	if second.Path != first.Path {
		t.Fatalf("相同内容应复用目标文件: first=%+v second=%+v", first, second)
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs"))
	if err != nil {
		t.Fatalf("读取上传目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("相同内容不应生成重复文件: %v", entries)
	}

	changed, err := UploadFileToRoot(root, "demo.txt", "docs/", strings.NewReader("changed content"))
	if err != nil {
		t.Fatalf("不同内容上传失败: %v", err)
	}
	if changed.Path == first.Path {
		t.Fatalf("不同内容不应复用原文件: first=%+v changed=%+v", first, changed)
	}
}

func assertWorkspaceFileContains(t *testing.T, root string, fileName string, expected string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, fileName))
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", fileName, err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("%s 缺少 %q: %s", fileName, expected, content)
	}
}

func assertNoStaleScheduleWakeupGuidance(t *testing.T, content string) {
	t.Helper()
	for _, stale := range []string{
		"ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒",
		"仅在**全部**满足时使用",
	} {
		if strings.Contains(content, stale) {
			t.Fatalf("AGENTS.md 仍包含旧 ScheduleWakeup 规则 %q: %s", stale, content)
		}
	}
}

func TestUploadFileToRootReusesAttachmentByMD5(t *testing.T) {
	root := t.TempDir()

	first, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-1/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("首次附件上传失败: %v", err)
	}
	second, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-2/", strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("重复附件上传失败: %v", err)
	}
	if second.Path != first.Path {
		t.Fatalf("附件相同内容应复用已有文件: first=%+v second=%+v", first, second)
	}
	if _, err = os.Stat(filepath.Join(root, "attachments", "batch-2", "demo.txt")); !os.IsNotExist(err) {
		t.Fatalf("重复附件不应落盘到新目录: %v", err)
	}

	changed, err := UploadFileToRoot(root, "demo.txt", "attachments/batch-3/", strings.NewReader("changed content"))
	if err != nil {
		t.Fatalf("不同附件上传失败: %v", err)
	}
	if changed.Path == first.Path {
		t.Fatalf("附件不同内容不应复用原文件: first=%+v changed=%+v", first, changed)
	}
}

func TestDeploySkillFallsBackToClaudeSkillMirrorWhenSymlinkUnavailable(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0o755); err != nil {
		t.Fatalf("创建 skill 源目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# {agent_name}\n"), 0o644); err != nil {
		t.Fatalf("写入 skill 模板失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "scripts", "run.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("写入 skill 附件失败: %v", err)
	}

	originalCreateSymlink := createSymlink
	createSymlink = func(string, string) error {
		return errors.New("symlink unavailable")
	}
	t.Cleanup(func() {
		createSymlink = originalCreateSymlink
	})

	workspacePath := filepath.Join(t.TempDir(), "workspace")
	renderContext := map[string]string{
		"agent_name":   "测试助手",
		"project_root": "/tmp/nexus",
		"workspace":    workspacePath,
	}
	if err := DeploySkill("demo-skill", sourceDir, workspacePath, renderContext); err != nil {
		t.Fatalf("部署 skill fallback 失败: %v", err)
	}

	claudeSkillDir := filepath.Join(workspacePath, ".claude", "skills", "demo-skill")
	if info, err := os.Lstat(claudeSkillDir); err != nil {
		t.Fatalf("Claude skill 镜像目录未生成: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("Claude skill fallback 应生成普通目录: mode=%s", info.Mode())
	}
	payload, err := os.ReadFile(filepath.Join(claudeSkillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("读取 Claude skill 镜像失败: %v", err)
	}
	if !strings.Contains(string(payload), "测试助手") {
		t.Fatalf("Claude skill 镜像未渲染模板: %s", payload)
	}
	if _, err = os.Stat(filepath.Join(workspacePath, ".agents", "skills", "demo-skill", "scripts", "run.txt")); err != nil {
		t.Fatalf(".agents skill 副本不完整: %v", err)
	}

	if err = UndeploySkill(workspacePath, "demo-skill"); err != nil {
		t.Fatalf("卸载 fallback skill 失败: %v", err)
	}
	if _, err = os.Stat(claudeSkillDir); !os.IsNotExist(err) {
		t.Fatalf("卸载后 Claude skill 镜像应被删除: %v", err)
	}
}

func TestServicePublishesWorkspaceLiveEvents(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "工作区实时助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	events := make(chan LiveEvent, 16)
	token, err := workspaceService.SubscribeLive(ctx, agentValue.AgentID, func(event LiveEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("订阅 workspace live 失败: %v", err)
	}
	defer workspaceService.UnsubscribeLive(token)
	time.Sleep(200 * time.Millisecond)

	if _, err = workspaceService.UpdateFile(ctx, agentValue.AgentID, "notes/live.md", "hello live"); err != nil {
		t.Fatalf("通过 API 更新文件失败: %v", err)
	}

	apiEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/live.md" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAPI
	})
	if apiEvent.ContentSnapshot == nil || *apiEvent.ContentSnapshot != "hello live" {
		t.Fatalf("API live 事件内容不正确: %+v", apiEvent)
	}

	agentFilePath := filepath.Join(agentValue.WorkspacePath, "notes", "agent.txt")
	if err = os.MkdirAll(filepath.Dir(agentFilePath), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err = os.WriteFile(agentFilePath, []byte("agent warmup"), 0o644); err != nil {
		t.Fatalf("模拟 agent 预热写文件失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = os.WriteFile(agentFilePath, []byte("agent write"), 0o644); err != nil {
		t.Fatalf("模拟 agent 写文件失败: %v", err)
	}

	agentEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/agent.txt" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAgent
	})
	if agentEvent.ContentSnapshot == nil || *agentEvent.ContentSnapshot != "agent write" {
		t.Fatalf("Agent live 事件内容不正确: %+v", agentEvent)
	}
}

func TestServiceFlushesWorkspaceLiveWrites(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	workspaceService := NewService(cfg, agentService)
	ctx := context.Background()

	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "写入结算助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	events := make(chan LiveEvent, 16)
	token, err := workspaceService.SubscribeLive(ctx, agentValue.AgentID, func(event LiveEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("订阅 workspace live 失败: %v", err)
	}
	defer workspaceService.UnsubscribeLive(token)
	time.Sleep(200 * time.Millisecond)

	agentFilePath := filepath.Join(agentValue.WorkspacePath, "notes", "flush.txt")
	if err = os.MkdirAll(filepath.Dir(agentFilePath), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err = os.WriteFile(agentFilePath, []byte("flush warmup"), 0o644); err != nil {
		t.Fatalf("模拟 agent 预热写文件失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = os.WriteFile(agentFilePath, []byte("flush now"), 0o644); err != nil {
		t.Fatalf("模拟 agent 写文件失败: %v", err)
	}
	_ = waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/flush.txt" &&
			event.Type == LiveEventFileWriteDelta &&
			event.Source == LiveSourceAgent &&
			event.ContentSnapshot != nil &&
			*event.ContentSnapshot == "flush now"
	})

	workspaceService.FlushLiveWrites(agentValue.AgentID)
	flushedEvent := waitWorkspaceLiveEvent(t, events, func(event LiveEvent) bool {
		return event.Path == "notes/flush.txt" &&
			event.Type == LiveEventFileWriteEnd &&
			event.Source == LiveSourceAgent
	})
	if flushedEvent.ContentSnapshot == nil || *flushedEvent.ContentSnapshot != "flush now" {
		t.Fatalf("强制结算 live 事件内容不正确: %+v", flushedEvent)
	}
}

func containsWorkspacePath(items []FileEntry, target string) bool {
	for _, item := range items {
		if item.Path == target {
			return true
		}
	}
	return false
}

func waitWorkspaceLiveEvent(t *testing.T, events <-chan LiveEvent, match func(LiveEvent) bool) LiveEvent {
	t.Helper()

	timeout := time.NewTimer(6 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case event := <-events:
			if match(event) {
				return event
			}
		case <-timeout.C:
			t.Fatal("等待 workspace live 事件超时")
		}
	}
}

func newWorkspaceTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:                      "127.0.0.1",
		Port:                      18011,
		ProjectName:               "nexus-workspace-test",
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

func migrateWorkspaceSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, workspaceTestMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func workspaceTestMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
