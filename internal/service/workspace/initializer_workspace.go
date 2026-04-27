package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/appfs"
)

var (
	baseSkillNames      = []string{"memory-manager", "room-collaboration"}
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
	for key, relativePath := range workspaceFiles {
		targetPath := filepath.Join(root, relativePath)
		if _, err := os.Stat(targetPath); err == nil {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		content := renderTemplate(workspaceTemplate(key, isMainAgent), context)
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

func managedSkillNames(isMainAgent bool) []string {
	items := append([]string{}, baseSkillNames...)
	if isMainAgent {
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

平台里有两种看起来像「定时」的工具，用途不同，不要混用，也**永远不要向用户介绍这两种类型**——用户只需描述需求：

- **nexus_automation（create_scheduled_task 等）= 产品级定时任务**
  用户能感知、能在「任务管理」页面看到、跨会话、需要持久或重复执行的都走这里。
  字段与 UI「新建任务」对话框一一对应（execution_mode / reply_mode / schedule 四种 kind：single/daily/interval/cron）。
  你只能 CRUD **自己 agent_id 名下**的任务，list 也只会看到自己的任务，越权操作会被后端拒绝。
  短文本提醒类任务可以只填 name+instruction+schedule，工具会默认按 temporary+none 创建；想让结果回当前会话才需要显式 existing+execution。

- **ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒**
  仅在**全部**满足时使用：一次性、延迟 < 30 分钟、只活在当前会话里、丢了不影响用户目标。
  这类节拍不会落到任务管理页面，用户看不到，也无法管理。

任何涉及「每天/每周/定时汇报/定时检查/定时提醒」的需求 → 一律走 create_scheduled_task。
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

## 定时任务路由

平台里有两种看起来像「定时」的工具，用途不同，不要混用，也**永远不要向用户介绍这两种类型**——用户只需描述需求：

- **nexus_automation（create_scheduled_task 等）= 产品级定时任务**
  用户能感知、能在「任务管理」页面看到、跨会话、需要持久或重复执行的都走这里。
  字段与 UI「新建任务」对话框一一对应（execution_mode / reply_mode / schedule 三种 kind）。
  作为主智能体，你不受 agent_id scope 限制，可以查看/管理任意智能体的任务；普通 Agent 只能 CRUD 自己的任务。
  遇到不确定的字段用 AskUserQuestion 问用户，禁止默认套值。

- **ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒**
  仅在**全部**满足时使用：一次性、延迟 < 30 分钟、只活在当前会话里、丢了不影响用户目标。
  这类"节拍"不会落到任务管理页面，用户看不到，也无法管理。

任何涉及"每天/每周/定时汇报/定时检查/定时提醒别人"的需求 → 一律走 create_scheduled_task。
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
