package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

var (
	baseSkillNames        = []string{"imagegen", "memory-manager", "scheduled-task-manager"}
	retiredBaseSkillNames = []string{"room-collaboration"}
	mainAgentSkillNames   = []string{"nexus-manager"}
	createSymlink         = os.Symlink
	workspaceFiles        = map[string]string{
		"agents":  "AGENTS.md",
		"user":    "USER.md",
		"memory":  "MEMORY.md",
		"soul":    "SOUL.md",
		"tools":   "TOOLS.md",
		"runbook": "RUNBOOK.md",
	}
	defaultDirs = []string{".agents", ".claude", "memory"}
)

// EnsureInitialized 保证 workspace 模板与系统技能已经落地。
func EnsureInitialized(
	agentID string,
	agentName string,
	workspacePath string,
	isMainAgent bool,
	createdAt time.Time,
) error {
	root := strings.TrimSpace(workspacePath)
	if root == "" {
		return fmt.Errorf("workspace_path 不能为空")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, dir := range defaultDirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}

	context := buildTemplateContext(agentID, agentName, root, createdAt)
	if err := ensureNexusctlShim(root, context); err != nil {
		return err
	}
	for key, relativePath := range workspaceFiles {
		targetPath := filepath.Join(root, relativePath)
		content := renderTemplate(workspaceTemplate(key, isMainAgent), context)
		if err := ensureWorkspaceTemplateFile(targetPath, key, content); err != nil {
			return err
		}
	}

	memoryReadmePath := filepath.Join(root, "memory", "README.md")
	if _, err := os.Stat(memoryReadmePath); os.IsNotExist(err) {
		if err = os.WriteFile(memoryReadmePath, []byte("# memory/\n\n存放按天日志、摘要、调研片段、临时结论和可复用资产。\n"), 0o644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	for _, skillName := range retiredBaseSkillNames {
		if err := UndeploySkill(root, skillName); err != nil {
			return err
		}
	}
	for _, skillName := range managedSkillNames(isMainAgent) {
		if err := deployManagedSkill(skillName, root, context); err != nil {
			return err
		}
	}
	return nil
}

// BuildSkillRenderContext 构建 skill 模板渲染上下文。
func BuildSkillRenderContext(agentID string, agentName string, workspacePath string, createdAt time.Time) map[string]string {
	return buildTemplateContext(agentID, agentName, workspacePath, createdAt)
}

// DeploySkill 把指定 skill 部署到目标 workspace。
func DeploySkill(skillName string, sourceDir string, workspacePath string, context map[string]string) error {
	agentsSkillDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	claudeSkillLink := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := syncDirectory(sourceDir, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureClaudeSkillEntry(sourceDir, claudeSkillLink, filepath.Join("..", "..", ".agents", "skills", skillName), context)
}

// UndeploySkill 从 workspace 中移除指定 skill。
func UndeploySkill(workspacePath string, skillName string) error {
	targetDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	claudeSkillLink := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.RemoveAll(claudeSkillLink); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListDeployedSkills 返回 workspace 当前已部署的全部 skill。
func ListDeployedSkills(workspacePath string) ([]string, error) {
	skillRoot := filepath.Join(workspacePath, ".agents", "skills")
	entries, err := os.ReadDir(skillRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			result = append(result, entry.Name())
		}
	}
	return result, nil
}

func managedSkillNames(isMainAgent bool) []string {
	items := append([]string{}, baseSkillNames...)
	if isMainAgent {
		items = append(items, mainAgentSkillNames...)
	}
	return items
}

func ensureWorkspaceTemplateFile(targetPath string, key string, content string) error {
	rendered := strings.TrimSpace(content)
	if rendered == "" {
		return nil
	}
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, []byte(rendered+"\n"), 0o644)
		}
		return err
	}
	if key != "agents" {
		return nil
	}
	return repairAgentsScheduleGuidance(targetPath, rendered+"\n")
}

func repairAgentsScheduleGuidance(targetPath string, rendered string) error {
	currentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	current := string(currentBytes)
	if !strings.Contains(current, "ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒") {
		return nil
	}
	repaired, ok := replaceMarkdownSection(current, rendered, []string{"## 定时任务路由", "## 定时任务"})
	if !ok || repaired == current {
		return nil
	}
	return os.WriteFile(targetPath, []byte(strings.TrimRight(repaired, "\n")+"\n"), 0o644)
}

func replaceMarkdownSection(current string, rendered string, headings []string) (string, bool) {
	for _, heading := range headings {
		currentStart, currentEnd, currentOK := markdownSectionBounds(current, heading)
		renderedStart, renderedEnd, renderedOK := markdownSectionBounds(rendered, heading)
		if !currentOK || !renderedOK {
			continue
		}
		return current[:currentStart] + rendered[renderedStart:renderedEnd] + current[currentEnd:], true
	}
	return "", false
}

func markdownSectionBounds(content string, heading string) (int, int, bool) {
	start := -1
	if strings.HasPrefix(content, heading+"\n") {
		start = 0
	} else if index := strings.Index(content, "\n"+heading+"\n"); index >= 0 {
		start = index + 1
	}
	if start < 0 {
		return 0, 0, false
	}
	searchFrom := start + len(heading) + 1
	if next := strings.Index(content[searchFrom:], "\n## "); next >= 0 {
		return start, searchFrom + next + 1, true
	}
	return start, len(content), true
}

func buildTemplateContext(agentID string, agentName string, workspacePath string, createdAt time.Time) map[string]string {
	timestamp := createdAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return map[string]string{
		"agent_id":     agentID,
		"agent_name":   agentName,
		"created_at":   timestamp.Format("2006-01-02 15:04:05"),
		"project_root": projectRoot(),
		"workspace":    filepath.Clean(workspacePath),
	}
}

func deployManagedSkill(skillName string, workspacePath string, context map[string]string) error {
	sourceDir := filepath.Join(projectRoot(), "skills", skillName)
	if _, err := os.Stat(filepath.Join(sourceDir, "SKILL.md")); err != nil {
		return err
	}
	return DeploySkill(skillName, sourceDir, workspacePath, context)
}

func syncDirectory(sourceDir string, targetDir string, context map[string]string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if filepath.Base(path) == "SKILL.md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rendered := renderTemplate(string(content), context)
			return os.WriteFile(targetPath, []byte(strings.TrimSpace(rendered)+"\n"), 0o644)
		}
		return copyFile(path, targetPath)
	})
}

func ensureNexusctlShim(workspacePath string, context map[string]string) error {
	binDir := filepath.Join(workspacePath, ".agents", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	content := renderTemplate(`#!/bin/sh
set -eu

PROJECT_ROOT="${NEXUS_PROJECT_ROOT:-{project_root}}"
CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

if [ -f "$PROJECT_ROOT/cmd/nexusctl/main.go" ]; then
  cd "$PROJECT_ROOT"
  exec go run ./cmd/nexusctl "$@"
fi

if [ -x "$PROJECT_ROOT/bin/nexusctl" ]; then
  exec "$PROJECT_ROOT/bin/nexusctl" "$@"
fi

if [ -x "$PROJECT_ROOT/bin/nexusctl.exe" ]; then
  exec "$PROJECT_ROOT/bin/nexusctl.exe" "$@"
fi

if [ -x "$PROJECT_ROOT/nexusctl" ]; then
  exec "$PROJECT_ROOT/nexusctl" "$@"
fi

if [ -x "$PROJECT_ROOT/nexusctl.exe" ]; then
  exec "$PROJECT_ROOT/nexusctl.exe" "$@"
fi

echo "nexusctl is unavailable: set NEXUS_PROJECT_ROOT or install nexusctl" >&2
exit 127
`, context)
	if err := os.WriteFile(filepath.Join(binDir, "nexusctl"), []byte(content), 0o755); err != nil {
		return err
	}
	cmdContent := renderTemplate(`@echo off
setlocal

set "PROJECT_ROOT=%NEXUS_PROJECT_ROOT%"
if "%PROJECT_ROOT%"=="" set "PROJECT_ROOT={project_root}"
set "CALLER_CWD=%CD%"
if "%NEXUSCTL_WORKSPACE_PATH%"=="" set "NEXUSCTL_WORKSPACE_PATH=%CALLER_CWD%"

if exist "%PROJECT_ROOT%\cmd\nexusctl\main.go" (
  cd /d "%PROJECT_ROOT%"
  go run ./cmd/nexusctl %*
  exit /b %ERRORLEVEL%
)

if exist "%PROJECT_ROOT%\bin\nexusctl.exe" (
  "%PROJECT_ROOT%\bin\nexusctl.exe" %*
  exit /b %ERRORLEVEL%
)

if exist "%PROJECT_ROOT%\nexusctl.exe" (
  "%PROJECT_ROOT%\nexusctl.exe" %*
  exit /b %ERRORLEVEL%
)

echo nexusctl is unavailable: set NEXUS_PROJECT_ROOT or install nexusctl 1>&2
exit /b 127
`, context)
	return os.WriteFile(filepath.Join(binDir, "nexusctl.cmd"), []byte(cmdContent), 0o755)
}

func ensureClaudeSkillEntry(sourceDir string, entryPath string, relativeTarget string, context map[string]string) error {
	err := ensureRelativeSymlink(entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给 Claude Code 读取。
	if mirrorErr := syncDirectory(sourceDir, entryPath, context); mirrorErr != nil {
		return fmt.Errorf("创建 Claude skill symlink 失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

func ensureRelativeSymlink(linkPath string, relativeTarget string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	if current, err := os.Readlink(linkPath); err == nil {
		if current == relativeTarget {
			return nil
		}
		if err = os.Remove(linkPath); err != nil {
			return err
		}
	} else if _, statErr := os.Stat(linkPath); statErr == nil {
		if err = os.RemoveAll(linkPath); err != nil {
			return err
		}
	}
	return createSymlink(relativeTarget, linkPath)
}

func copyFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	return os.Chmod(targetPath, info.Mode())
}

func renderTemplate(raw string, context map[string]string) string {
	replacerArgs := make([]string, 0, len(context)*2)
	for key, value := range context {
		replacerArgs = append(replacerArgs, "{"+key+"}", value)
	}
	return strings.NewReplacer(replacerArgs...).Replace(raw)
}

func projectRoot() string {
	return appfs.Root()
}

func workspaceTemplate(key string, isMainAgent bool) string {
	if isMainAgent {
		return mainAgentWorkspaceTemplates[key]
	}
	return defaultWorkspaceTemplates[key]
}

var defaultWorkspaceTemplates = map[string]string{
	"agents": `# AGENTS.md

## Agent Profile

你是 Nexus，一个由 Nexus Research Lab 创造的智能助手。

当前 Agent 标识：{agent_name}（{agent_id}）

工作区在 {workspace}，你只能在限制的工作区内工作。

默认语言：中文
工作方式：先明确目标，再执行，再回传结果
事实原则：不编造，结论有依据，不确定就说明边界

## 定时任务

用户凡是提出「提醒我...」「几分钟后...」「每天/每周...」「定时检查/汇报/投递」等用户可见的提醒或定时任务，都必须创建 Nexus 持久化任务。

- **nexus_automation（create_scheduled_task 等）= 唯一用户可见定时任务入口**
  用户能感知、能在「任务管理」页面看到、跨会话、需要持久或重复执行的都走这里。
  字段与 UI「新建任务」对话框一一对应（execution_mode / reply_mode / schedule 四种 kind：single/daily/interval/cron）。
  你只能 CRUD **自己 agent_id 名下**的任务，list 也只会看到自己的任务，越权操作会被后端拒绝。
  短文本提醒类任务也走 create_scheduled_task：可以只填 name+instruction+schedule，工具会默认按 existing+execution 创建可见提醒；日报、监控、飞书群投递和检查发送情况必须先加载 scheduled-task-manager。

不要用 ScheduleWakeup、Cron harness 或会话内临时 wakeup 承诺/交付用户提醒；这些即使在运行环境里出现，也只属于运行时自我续跑机制，不会进入任务管理，不可查询、不可停止、不可补发，丢失后用户目标会失败。
不要向用户解释工具差异；用户只需描述需求，你负责把它落成可管理的任务。
`,
	"user": `# USER.md

## 用户偏好

- 常用语言：
- 回复风格：
- 不希望出现的表达：
- 当前重点：
`,
	"memory": `# MEMORY.md

## 长期记忆

- 偏好：
- 约束：
- 决策记录：
`,
	"soul": `# SOUL.md

## 行为准则

- 复杂任务前先看近期日记，避免重复犯错。
- 用户明确表达的偏好和长期规则，立即提升为稳定记忆。
`,
	"tools": `# TOOLS.md

## 工具备忘

- 记录命令、接口、外部服务的限制和坑点。
`,
	"runbook": `# RUNBOOK.md

## 工作手册

创建时间：{created_at}

### 当前项目上下文
- 项目：
- 目标：
- 约束：
`,
}

var mainAgentWorkspaceTemplates = map[string]string{
	"agents": `# AGENTS.md

## Main Agent Profile

你是“Nexus”，是系统级组织代理，不是普通 room 成员。

当前 Agent 标识：{agent_name}（{agent_id}）

你的职责：
- 理解用户当前要推进的协作目标
- 整理任务、成员、上下文与下一步建议
- 决定是恢复已有 Room，还是创建新的 Room
- 在必要时把用户带到合适的 Room 或 Contacts

## 定时任务路由

用户凡是提出「提醒我...」「几分钟后...」「每天/每周...」「定时检查/汇报/投递」等用户可见的提醒或定时任务，都必须创建 Nexus 持久化任务。

- **nexus_automation（create_scheduled_task 等）= 唯一用户可见定时任务入口**
  用户能感知、能在「任务管理」页面看到、跨会话、需要持久或重复执行的都走这里。
  字段与 UI「新建任务」对话框一一对应（execution_mode / reply_mode / schedule 四种 kind：single/daily/interval/cron）。
  作为主智能体，你不受 agent_id scope 限制，可以查看/管理任意智能体的任务；普通 Agent 只能 CRUD 自己的任务。
  短文本提醒类任务也走 create_scheduled_task：可以只填 name+instruction+schedule，工具会默认按 existing+execution 创建可见提醒。
  遇到不确定字段必须先向用户确认，禁止默认套值；在网页/桌面会话可用 AskUserQuestion，在飞书/IM 等外部通道用普通文本回复让用户补充。检查发送情况、恢复卡住任务、补发投递失败时必须使用 scheduled-task-manager 里的工具顺序。

不要用 ScheduleWakeup、Cron harness 或会话内临时 wakeup 承诺/交付用户提醒；这些即使在运行环境里出现，也只属于运行时自我续跑机制，不会进入任务管理，不可查询、不可停止、不可补发，丢失后用户目标会失败。
不要向用户解释工具差异；用户只需描述需求，你负责把它落成可管理的任务。
`,
	"user": defaultWorkspaceTemplates["user"],
	"memory": `# MEMORY.md

## 长期记忆

- 用户希望首页中的 Nexus 是唯一系统级 agent
- Nexus 应负责组织协作，而不是替代 Room 承载执行
`,
	"soul": defaultWorkspaceTemplates["soul"],
	"tools": `# TOOLS.md

## 工具备忘

- 记录创建 agent、创建 room、管理 skill 的稳定用法。
`,
	"runbook": `# RUNBOOK.md

## Main Agent Runbook

创建时间：{created_at}

### 你的固定任务
- 识别当前请求更适合恢复已有协作还是创建新协作
- 当需要多人协作时，先组织成员和结构，再引导进入 Room
`,
}
