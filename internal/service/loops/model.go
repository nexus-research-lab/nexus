package loops

import (
	"slices"
	"strings"
)

// Loop 表示可复制到聊天里的预制 agent loop。
type Loop struct {
	ID               string         `json:"id"`
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	TitleZH          string         `json:"title_zh,omitempty"`
	Description      string         `json:"description"`
	DescriptionZH    string         `json:"description_zh,omitempty"`
	Category         string         `json:"category"`
	CategoryZH       string         `json:"category_zh,omitempty"`
	TriggerType      string         `json:"trigger_type"`
	TriggerConfig    map[string]any `json:"trigger_config"`
	Steps            []Step         `json:"steps"`
	ExitCondition    ExitCondition  `json:"exit_condition"`
	KickoffPrompt    string         `json:"kickoff_prompt"`
	InstallBundle    map[string]any `json:"install_bundle"`
	CompatibleAgents []string       `json:"compatible_agents"`
	BestForAgents    []string       `json:"best_for_agents"`
	Author           string         `json:"author"`
	AuthorSlug       string         `json:"author_slug"`
	AuthorOfficial   bool           `json:"author_official"`
	Source           string         `json:"source"`
	Tags             []string       `json:"tags"`
	Guardrails       []string       `json:"guardrails"`
	GuardrailsZH     []string       `json:"guardrails_zh,omitempty"`
	Examples         []Example      `json:"examples"`
	Copies           int            `json:"copies"`
	Installs         int            `json:"installs"`
	Views            int            `json:"views"`
	Featured         bool           `json:"featured"`
	IsPublished      bool           `json:"is_published"`
	CreatedAt        string         `json:"created_at"`
}

// Step 表示 loop 的单个执行步骤。
type Step struct {
	Name       string `json:"name"`
	NameZH     string `json:"name_zh,omitempty"`
	Prompt     string `json:"prompt"`
	PromptZH   string `json:"prompt_zh,omitempty"`
	ShellCheck string `json:"shell_check,omitempty"`
}

// ExitCondition 表示 loop 的结束条件。
type ExitCondition struct {
	Type          string `json:"type"`
	Command       string `json:"command,omitempty"`
	Description   string `json:"description"`
	DescriptionZH string `json:"description_zh,omitempty"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// Example 表示 loop 示例。
type Example struct {
	Title     string `json:"title"`
	TitleZH   string `json:"title_zh,omitempty"`
	Summary   string `json:"summary"`
	SummaryZH string `json:"summary_zh,omitempty"`
}

// Localized 返回适合目标语言展示的副本；kickoff_prompt 保持原文，保证可复制执行。
func (l Loop) Localized(locale string) Loop {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return l
	}
	if l.TitleZH != "" {
		l.Title = l.TitleZH
	}
	if l.DescriptionZH != "" {
		l.Description = l.DescriptionZH
	}
	if l.CategoryZH != "" {
		l.Category = l.CategoryZH
	}
	l.Steps = slices.Clone(l.Steps)
	for i := range l.Steps {
		if l.Steps[i].NameZH != "" {
			l.Steps[i].Name = l.Steps[i].NameZH
		}
		if l.Steps[i].PromptZH != "" {
			l.Steps[i].Prompt = l.Steps[i].PromptZH
		}
	}
	if l.ExitCondition.DescriptionZH != "" {
		l.ExitCondition.Description = l.ExitCondition.DescriptionZH
	}
	if len(l.GuardrailsZH) > 0 {
		l.Guardrails = slices.Clone(l.GuardrailsZH)
	}
	l.Examples = slices.Clone(l.Examples)
	for i := range l.Examples {
		if l.Examples[i].TitleZH != "" {
			l.Examples[i].Title = l.Examples[i].TitleZH
		}
		if l.Examples[i].SummaryZH != "" {
			l.Examples[i].Summary = l.Examples[i].SummaryZH
		}
	}
	return l
}
