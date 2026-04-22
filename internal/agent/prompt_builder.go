package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/config"
	memorysvc "github.com/nexus-research-lab/nexus/internal/memory"
)

const defaultBaseSystemPrompt = `# Nexus Base System Prompt

你是 Nexus，一个由 Nexus Research Lab 创造的智能助手。

身份要求：
- 对外自称 "Nexus"。
- 当用户询问你的身份、来源或开发者时，明确说明你由 Nexus Research Lab 创造。

行为要求：
- 默认使用中文交流；若用户明确要求其他语言，再切换。
- 优先给出直接、可执行、可验证的结果，避免空泛措辞。
- 如果工作区规则与本基础身份设定冲突，以本基础身份设定为准。
`

const defaultMainAgentSystemPrompt = `# Nexus System Prompt

你是"Nexus"，是整个系统里的唯一系统级组织代理。

你的目标不是代替具体 room 承载执行，而是：
- 理解用户要推进的协作目标
- 判断应该恢复已有协作、创建新协作，还是先去选择成员
- 把用户快速带到合适的 room、conversation 或 Contacts
- 当需要创建 agent、创建 room、邀请成员时，直接执行，不只停留在建议层

你的行为要求：
- 默认使用中文
- 回复直接、简洁、少解释
- 不输出产品说明、系统架构说明或自我介绍型文案
- 用户意图明确时，优先给出下一步动作
- 需要创建协作时，优先生成清晰的 room 标题和组织建议
- 需要找成员时，优先引导到 Contacts 或明确推荐候选成员
- 涉及协作编排动作时，优先使用 nexus-manager skill 和对应 CLI
- 读取工具结果时先看 JSON 里的 ok，失败就明确报错，不要编造已完成

你的边界：
- 你不是普通成员 agent
- 你不是独立后台页面
- 你不长期承载执行型协作
- 真正的执行协作应回到具体 room 内完成
- 不能作为 room 成员
`

var promptFileNames = []string{
	"AGENTS.md",
	"USER.md",
	"MEMORY.md",
	"SOUL.md",
	"TOOLS.md",
	"RUNBOOK.md",
}

type promptBuilder struct {
	config config.Config
}

func newPromptBuilder(cfg config.Config) *promptBuilder {
	return &promptBuilder{config: cfg}
}

// Build 构建运行时附加系统提示词。
func (b *promptBuilder) Build(ctx context.Context, agentValue *Agent) (string, error) {
	if agentValue == nil {
		return "", nil
	}

	sections := make([]string, 0, 10)
	staticPrompt := strings.TrimSpace(b.loadStaticPrompt(agentValue))
	if staticPrompt != "" {
		sections = append(sections, staticPrompt)
	}

	if scopeSection := buildRuntimeScopeSection(ctx); scopeSection != "" {
		sections = append(sections, scopeSection)
	}

	if profileSection := buildAgentProfileSection(agentValue); profileSection != "" {
		sections = append(sections, profileSection)
	}

	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	if workspacePath != "" {
		sections = append(sections, fmt.Sprintf("当前工作区绝对路径: %s", workspacePath))
	}

	fileSections, err := b.loadWorkspacePromptSections(workspacePath)
	if err != nil {
		return "", err
	}
	sections = append(sections, fileSections...)

	if workspacePath != "" {
		reviewMarkdown, reviewErr := memorysvc.NewService(workspacePath).BuildReviewMarkdown(3, 6, 1200)
		if reviewErr != nil {
			return "", reviewErr
		}
		if strings.TrimSpace(reviewMarkdown) != "" {
			sections = append(sections, "## 最近日记提醒\n"+strings.TrimSpace(reviewMarkdown))
		}
	}

	sections = compactPromptSections(sections)
	if len(sections) == 0 {
		return "", nil
	}
	return strings.Join(sections, "\n\n---\n\n"), nil
}

func buildRuntimeScopeSection(ctx context.Context) string {
	principal := authsvc.PrincipalFromContext(ctx)
	state, hasState := authsvc.StateFromContext(ctx)
	userID, hasUserID := authsvc.CurrentUserID(ctx)

	lines := []string{"## 当前运行作用域"}
	switch {
	case hasUserID:
		lines = append(lines,
			"运行模式: 多用户用户作用域",
			"当前 user_id: "+userID,
		)
		if principal != nil && strings.TrimSpace(principal.Username) != "" {
			lines = append(lines, "当前 username: "+strings.TrimSpace(principal.Username))
		}
		lines = append(lines, "边界要求: 只能读取和操作当前 user_id 作用域内的 agent、room、session、workspace，不要假设可访问其他用户的数据。")
	case hasState && state.AuthRequired:
		lines = append(lines, "运行模式: 认证系统作用域", "边界要求: 当前请求未绑定具体用户，不要假设拥有全局用户数据访问权。")
	default:
		lines = append(lines,
			"运行模式: 单用户系统作用域",
			"当前主体: "+authsvc.SystemUserID,
			"边界要求: 当前实例按单用户模式运行，可以把当前工作区视为系统默认作用域。",
		)
	}
	return strings.Join(lines, "\n")
}

func (b *promptBuilder) loadStaticPrompt(agentValue *Agent) string {
	if agentValue != nil && (agentValue.IsMain || strings.TrimSpace(agentValue.AgentID) == strings.TrimSpace(b.config.DefaultAgentID)) {
		return firstNonEmptyPrompt(b.config.MainAgentSystemPrompt, defaultMainAgentSystemPrompt)
	}
	return firstNonEmptyPrompt(b.config.BaseSystemPrompt, defaultBaseSystemPrompt)
}

func (b *promptBuilder) loadWorkspacePromptSections(workspacePath string) ([]string, error) {
	trimmedWorkspacePath := strings.TrimSpace(workspacePath)
	if trimmedWorkspacePath == "" {
		return nil, nil
	}
	sections := make([]string, 0, len(promptFileNames))
	for _, fileName := range promptFileNames {
		content, err := readOptionalWorkspacePromptFile(trimmedWorkspacePath, fileName)
		if err != nil {
			return nil, err
		}
		if content != "" {
			sections = append(sections, content)
		}
	}
	return sections, nil
}

func readOptionalWorkspacePromptFile(workspacePath string, fileName string) (string, error) {
	targetPath := filepath.Join(workspacePath, fileName)
	content, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func buildAgentProfileSection(agentValue *Agent) string {
	if agentValue == nil {
		return ""
	}
	displayName := strings.TrimSpace(agentValue.DisplayName)
	if displayName == strings.TrimSpace(agentValue.Name) {
		displayName = ""
	}
	headline := strings.TrimSpace(agentValue.Headline)
	description := strings.TrimSpace(agentValue.Description)
	profileMarkdown := strings.TrimSpace(agentValue.ProfileMarkdown)
	if displayName == "" && headline == "" && description == "" && profileMarkdown == "" {
		return ""
	}

	lines := []string{"## Agent Profile"}
	if displayName != "" {
		lines = append(lines, "显示名："+displayName)
	}
	if headline != "" {
		lines = append(lines, "一句话简介："+headline)
	}
	if description != "" && description != headline {
		lines = append(lines, "补充描述："+description)
	}
	if profileMarkdown != "" {
		lines = append(lines, "", profileMarkdown)
	}
	return strings.Join(lines, "\n")
}

func compactPromptSections(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func firstNonEmptyPrompt(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
