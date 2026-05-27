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
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	baseSkillNames        = []string{"imagegen", "memory-manager", "scheduled-task-manager"}
	retiredBaseSkillNames = []string{"room-collaboration"}
	mainAgentSkillNames   = []string{"nexus-manager"}
	createSymlink         = os.Symlink
	workspaceFiles        = map[string]string{
		"agents": "AGENTS.md",
		"user":   "USER.md",
		"memory": "MEMORY.md",
		"soul":   "SOUL.md",
		"tools":  "TOOLS.md",
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
	if err := agentsvc.EnsureRuntimeEmotionState(root); err != nil {
		return err
	}

	context := buildTemplateContext(agentID, agentName, root, createdAt)
	if err := ensureNexusctlShim(appfs.AgentRuntimeBinDir(), context); err != nil {
		return err
	}
	if err := removeWorkspaceBinShim(root); err != nil {
		return err
	}
	for key, relativePath := range workspaceFiles {
		targetPath := filepath.Join(root, relativePath)
		if isMainAgent && key == "agents" {
			if err := removeGeneratedMainAgentsPrompt(targetPath); err != nil {
				return err
			}
			if _, err := os.Stat(targetPath); err == nil {
				if err := repairAgentsScheduleGuidance(targetPath); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if isMainAgent && (key == "soul" || key == "tools") {
			if err := removeGeneratedMainWorkspaceFile(targetPath); err != nil {
				return err
			}
			continue
		}
		content := renderTemplate(workspaceTemplate(key, isMainAgent), context)
		if err := ensureWorkspaceTemplateFile(targetPath, key, content); err != nil {
			return err
		}
	}

	memoryReadmePath := filepath.Join(root, "memory", "README.md")
	if _, err := os.Stat(memoryReadmePath); os.IsNotExist(err) {
		if err = os.WriteFile(memoryReadmePath, []byte("# memory/\n\nDaily notes, summaries, research fragments, temporary conclusions, and reusable memory assets live here.\n"), 0o644); err != nil {
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
	return repairAgentsScheduleGuidance(targetPath)
}

func repairAgentsScheduleGuidance(targetPath string) error {
	// TODO: 迁移期清理旧 AGENTS.md 里的 ScheduleWakeup 说明；确认旧 workspace 已覆盖后删除。
	currentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	current := string(currentBytes)
	if !strings.Contains(current, "ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒") {
		return nil
	}
	repaired, ok := removeMarkdownSection(current, []string{"## 定时任务路由", "## 定时任务", "## Scheduled Task Routing", "## Scheduled Tasks"})
	if !ok || repaired == current {
		return nil
	}
	return os.WriteFile(targetPath, []byte(strings.TrimRight(repaired, "\n")+"\n"), 0o644)
}

func removeGeneratedMainAgentsPrompt(targetPath string) error {
	contentBytes, err := os.ReadFile(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !looksLikeGeneratedMainAgentsPrompt(content) {
		return nil
	}
	return os.Remove(targetPath)
}

func removeGeneratedMainWorkspaceFile(targetPath string) error {
	contentBytes, err := os.ReadFile(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !looksLikeGeneratedMainWorkspaceFile(filepath.Base(targetPath), content) {
		return nil
	}
	return os.Remove(targetPath)
}

func looksLikeGeneratedMainAgentsPrompt(content string) bool {
	return strings.Contains(content, "## Main Agent Profile") &&
		(strings.Contains(content, "system-level collaboration organizer") || strings.Contains(content, "系统级组织代理"))
}

func looksLikeGeneratedMainWorkspaceFile(fileName string, content string) bool {
	switch fileName {
	case "SOUL.md":
		return strings.Contains(content, "## Personality") && strings.Contains(content, "## Emotion")
	case "TOOLS.md":
		return strings.Contains(content, "## Tool Notes") && strings.Contains(content, "## Skill Notes")
	default:
		return false
	}
}

func removeMarkdownSection(current string, headings []string) (string, bool) {
	for _, heading := range headings {
		currentStart, currentEnd, currentOK := markdownSectionBounds(current, heading)
		if !currentOK {
			continue
		}
		return current[:currentStart] + current[currentEnd:], true
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

func ensureNexusctlShim(binDir string, context map[string]string) error {
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

func removeWorkspaceBinShim(workspacePath string) error {
	// TODO: 迁移期清理旧 per-agent / per-owner nexusctl shim；确认旧版本用户已覆盖后删除。
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	for _, binDir := range []string{
		filepath.Join(root, ".agents", "bin"),
		filepath.Join(filepath.Dir(root), ".agents", "bin"),
	} {
		if err := removeGeneratedNexusctlBinDir(binDir); err != nil {
			return err
		}
	}
	return nil
}

func removeGeneratedNexusctlBinDir(binDir string) error {
	if filepath.Clean(binDir) == filepath.Clean(appfs.AgentRuntimeBinDir()) {
		return nil
	}
	for _, fileName := range []string{"nexusctl", "nexusctl.cmd"} {
		targetPath := filepath.Join(binDir, fileName)
		content, err := os.ReadFile(targetPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !looksLikeGeneratedNexusctlShim(string(content)) {
			continue
		}
		if err = os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return removeDirIfEmpty(binDir)
}

func looksLikeGeneratedNexusctlShim(content string) bool {
	return strings.Contains(content, "NEXUSCTL_WORKSPACE_PATH") &&
		strings.Contains(content, "nexusctl is unavailable: set NEXUS_PROJECT_ROOT or install nexusctl")
}

func removeDirIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	if err = os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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
