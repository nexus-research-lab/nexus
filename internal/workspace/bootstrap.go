// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：bootstrap.go
// @Date   ：2026/04/11 14:58:00
// @Author ：leemysw
// 2026/04/11 14:58:00   Create
// =====================================================

package workspace

import (
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	baseSkillNames      = []string{"memory-manager"}
	mainAgentSkillNames = []string{"nexus-manager"}
	workspaceFiles      = map[string]string{
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
func EnsureInitialized(cfg config.Config, agentID string, agentName string, workspacePath string, createdAt time.Time) error {
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
	for key, relativePath := range workspaceFiles {
		targetPath := filepath.Join(root, relativePath)
		if _, err := os.Stat(targetPath); err == nil {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		content := renderTemplate(workspaceTemplate(key, agentID == cfg.DefaultAgentID), context)
		if strings.TrimSpace(content) == "" {
			continue
		}
		if err := os.WriteFile(targetPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
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

	for _, skillName := range managedSkillNames(cfg, agentID) {
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
	return ensureRelativeSymlink(claudeSkillLink, filepath.Join("..", "..", ".agents", "skills", skillName))
}

// UndeploySkill 从 workspace 中移除指定 skill。
func UndeploySkill(workspacePath string, skillName string) error {
	targetDir := filepath.Join(workspacePath, ".agents", "skills", skillName)
	claudeSkillLink := filepath.Join(workspacePath, ".claude", "skills", skillName)
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.Remove(claudeSkillLink); err != nil && !os.IsNotExist(err) {
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

func managedSkillNames(cfg config.Config, agentID string) []string {
	items := append([]string{}, baseSkillNames...)
	if strings.TrimSpace(agentID) == strings.TrimSpace(cfg.DefaultAgentID) {
		items = append(items, mainAgentSkillNames...)
	}
	return items
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
	} else if err == nil {
		// no-op
	} else if _, statErr := os.Stat(linkPath); statErr == nil {
		if err = os.RemoveAll(linkPath); err != nil {
			return err
		}
	}
	return os.Symlink(relativeTarget, linkPath)
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
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
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
- 决定是恢复已有 room，还是创建新的 room
- 在必要时把用户带到合适的 room 或 Contacts
`,
	"user": defaultWorkspaceTemplates["user"],
	"memory": `# MEMORY.md

## 长期记忆

- 用户希望首页中的 Nexus 是唯一系统级 agent
- Nexus 应负责组织协作，而不是替代 room 承载执行
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
- 当需要多人协作时，先组织成员和结构，再引导进入 room
`,
}
