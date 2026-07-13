package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEnsureNexusctlShimUsesExplicitCommandPath(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "shared-bin")
	commandPath := filepath.Join(root, "tools", "nexusctl")
	if err := os.MkdirAll(filepath.Dir(commandPath), 0o755); err != nil {
		t.Fatalf("创建 nexusctl 目录失败: %v", err)
	}
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("写入 nexusctl 可执行文件失败: %v", err)
	}
	t.Setenv("NEXUSCTL_COMMAND_PATH", commandPath)

	if err := ensureNexusctlShim(binDir, map[string]string{"project_root": filepath.Join(root, "project")}); err != nil {
		t.Fatalf("生成 nexusctl shim 失败: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(binDir, "nexusctl"))
	if err != nil {
		t.Fatalf("读取 nexusctl shim 失败: %v", err)
	}
	content := string(payload)
	if !strings.Contains(content, shellSingleQuote(commandPath)) {
		t.Fatalf("nexusctl shim 未绑定显式命令路径: %s", content)
	}
	if strings.Contains(content, "go run ./cmd/nexusctl") || strings.Contains(content, "bin/nexusctl") {
		t.Fatalf("显式 nexusctl shim 不应包含源码或打包 fallback: %s", content)
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
		"SOUL.md":   "## Emotion",
		"TOOLS.md":  "## Tool Notes",
	} {
		assertWorkspaceFileContains(t, root, fileName, expected)
	}
	for _, fileName := range []string{"MEMORY.md", "RUNBOOK.md"} {
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
	for _, fileName := range []string{"MEMORY.md", "SOUL.md", "TOOLS.md", "RUNBOOK.md"} {
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

func TestDeploySkillFallsBackToLegacySkillMirrorWhenSymlinkUnavailable(t *testing.T) {
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

	legacySkillDir := filepath.Join(workspacePath, ".claude", "skills", "demo-skill")
	if info, err := os.Lstat(legacySkillDir); err != nil {
		t.Fatalf("legacy skill 镜像目录未生成: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("legacy skill fallback 应生成普通目录: mode=%s", info.Mode())
	}
	payload, err := os.ReadFile(filepath.Join(legacySkillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("读取 legacy skill 镜像失败: %v", err)
	}
	if !strings.Contains(string(payload), "测试助手") {
		t.Fatalf("legacy skill 镜像未渲染模板: %s", payload)
	}
	if _, err = os.Stat(filepath.Join(workspacePath, ".agents", "skills", "demo-skill", "scripts", "run.txt")); err != nil {
		t.Fatalf(".agents skill 副本不完整: %v", err)
	}

	if err = UndeploySkill(workspacePath, "demo-skill"); err != nil {
		t.Fatalf("卸载 fallback skill 失败: %v", err)
	}
	if _, err = os.Stat(legacySkillDir); !os.IsNotExist(err) {
		t.Fatalf("卸载后 legacy skill 镜像应被删除: %v", err)
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
