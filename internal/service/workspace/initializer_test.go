package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func TestWorkspaceBrowserHidesAgentProfileTemplate(t *testing.T) {
	for _, testCase := range []string{"AGENTS.md", "agents.md"} {
		if !shouldHideWorkspaceBrowserEntry(testCase) {
			t.Fatalf("Agent 身份文件应从 workspace 文件树隐藏: %s", testCase)
		}
	}
	for _, testCase := range []string{"USER.md", "nested/AGENTS.md", "tmp/attachments/AGENTS.md"} {
		if shouldHideWorkspaceBrowserEntry(testCase) {
			t.Fatalf("不应隐藏普通或嵌套 workspace 文件: %s", testCase)
		}
	}
}

func TestEnsureInitializedWritesPromptLayerTemplates(t *testing.T) {
	useTemporaryWorkspaceStateRoot(t)
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
	for _, fileName := range []string{"AGENTS.md", "USER.md", "SOUL.md", "TOOLS.md"} {
		content, err := os.ReadFile(filepath.Join(root, fileName))
		if err != nil {
			t.Fatalf("读取 workspace 模板 %s 失败: %v", fileName, err)
		}
		if strings.HasPrefix(strings.TrimSpace(string(content)), "# "+fileName) {
			t.Fatalf("workspace 模板不应在文件内容开头注入文件名 %s: %s", fileName, content)
		}
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

func TestEnsureInitializedSerializesConcurrentWorkspaceInitialization(t *testing.T) {
	useTemporaryWorkspaceStateRoot(t)
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
		if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", skillName)); !os.IsNotExist(err) {
			t.Fatalf("平台 Skill 不应在 workspace 生成副本 %s: %v", skillName, err)
		}
	}
}

func TestEnsureInitializedRemovesBundledSkillCopies(t *testing.T) {
	useTemporaryWorkspaceStateRoot(t)
	root := t.TempDir()
	for _, name := range []string{"ima-skill", "imagegen"} {
		skillPath := filepath.Join(root, ".agents", "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
			t.Fatalf("创建旧平台 Skill 副本失败: %v", err)
		}
		if err := os.WriteFile(skillPath, []byte("stale"), 0o644); err != nil {
			t.Fatalf("写入旧平台 Skill 副本失败: %v", err)
		}
	}
	if err := EnsureInitialized("agent-1", "Planner", root, false, time.Now()); err != nil {
		t.Fatalf("初始化 workspace 失败: %v", err)
	}
	for _, name := range []string{"ima-skill", "imagegen"} {
		if _, err := os.Stat(filepath.Join(root, ".agents", "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("平台 Skill 副本未清理 %s: %v", name, err)
		}
	}
}

func TestRuntimeSkillNamesKeepsWorkspaceDeployedSkills(t *testing.T) {
	workspacePath := t.TempDir()
	for _, relative := range []string{
		filepath.Join(".agents", "skills", "external-skill", "SKILL.md"),
		filepath.Join(".agents", "workspace-local", "SKILL.md"),
		filepath.Join(".claude", "skills", "claude-only", "SKILL.md"),
		filepath.Join(".claude", "skills", "IMAGEGEN", "SKILL.md"),
	} {
		path := filepath.Join(workspacePath, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("创建 Skill 目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte("---\nname: test\n---\n"), 0o644); err != nil {
			t.Fatalf("写入 Skill 文件失败: %v", err)
		}
	}

	got, err := RuntimeSkillNames(workspacePath, []string{"imagegen", "external:ima-skill"})
	if err != nil {
		t.Fatalf("合并 runtime Skill 名称失败: %v", err)
	}
	want := []string{"imagegen", "ima-skill", "claude-only", "external-skill", "workspace-local"}
	if !slices.Equal(got, want) {
		t.Fatalf("runtime Skill 名称 = %#v，期望 %#v", got, want)
	}
}

func TestEnsureInitializedRepairsStaleScheduleWakeupGuidance(t *testing.T) {
	useTemporaryWorkspaceStateRoot(t)
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

func useTemporaryWorkspaceStateRoot(t *testing.T) {
	t.Helper()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(t.TempDir(), ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", "")
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
		t.Fatalf("Claude Skill 镜像目录未生成: %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("Claude Skill fallback 应生成普通目录: mode=%s", info.Mode())
	}
	payload, err := os.ReadFile(filepath.Join(claudeSkillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("读取 Claude Skill 镜像失败: %v", err)
	}
	if !strings.Contains(string(payload), "测试助手") {
		t.Fatalf("Claude Skill 镜像未渲染模板: %s", payload)
	}
	if _, err = os.Stat(filepath.Join(workspacePath, ".agents", "skills", "demo-skill", "scripts", "run.txt")); err != nil {
		t.Fatalf(".agents skill 副本不完整: %v", err)
	}

	if err = UndeploySkill(workspacePath, "demo-skill"); err != nil {
		t.Fatalf("卸载 fallback skill 失败: %v", err)
	}
	if _, err = os.Stat(claudeSkillDir); !os.IsNotExist(err) {
		t.Fatalf("卸载后 Claude Skill 镜像应被删除: %v", err)
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
