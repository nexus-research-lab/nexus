package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var defaultWorkspacePromptFiles = []string{
	"AGENTS.md",
	"USER.md",
	"MEMORY.md",
	"SOUL.md",
	"TOOLS.md",
}

var mainAgentWorkspacePromptFiles = []string{
	"USER.md",
	"MEMORY.md",
}

type promptBuilder struct {
	config config.Config
}

type promptBuildScope struct {
	isMainAgent   bool
	workspacePath string
}

func newPromptBuilder(cfg config.Config) *promptBuilder {
	return &promptBuilder{config: cfg}
}

// Build 构建运行时附加系统提示词。
func (b *promptBuilder) Build(ctx context.Context, agentValue *protocol.Agent) (string, error) {
	if agentValue == nil {
		return "", nil
	}

	scope := b.newBuildScope(agentValue)
	sections := make([]string, 0, 8)
	sections = appendPromptSection(sections, b.loadStaticPrompt(scope))
	sections = appendPromptSection(sections, buildRuntimeScopeSection(ctx))
	sections = appendPromptSection(sections, buildAgentProfileSection(agentValue, scope))

	fileSections, err := loadWorkspacePromptSections(scope)
	if err != nil {
		return "", err
	}
	for _, section := range fileSections {
		sections = appendPromptSection(sections, section)
	}
	if len(sections) == 0 {
		return "", nil
	}
	return strings.Join(sections, "\n\n---\n\n"), nil
}

func (b *promptBuilder) newBuildScope(agentValue *protocol.Agent) promptBuildScope {
	workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
	if workspacePath == "" {
		workspacePath = ResolveWorkspacePath(b.config, agentValue.OwnerUserID, agentValue.AgentID)
	}
	return promptBuildScope{
		isMainAgent:   isMainAgentPrompt(agentValue, b.config.DefaultAgentID),
		workspacePath: workspacePath,
	}
}

func (b *promptBuilder) loadStaticPrompt(scope promptBuildScope) string {
	if scope.isMainAgent {
		return firstNonEmptyPrompt(b.config.MainAgentSystemPrompt, defaultMainAgentSystemPrompt)
	}
	return firstNonEmptyPrompt(b.config.BaseSystemPrompt, defaultBaseSystemPrompt)
}

func (scope promptBuildScope) workspacePromptFiles() []string {
	if scope.isMainAgent {
		return mainAgentWorkspacePromptFiles
	}
	return defaultWorkspacePromptFiles
}

func appendPromptSection(sections []string, section string) []string {
	section = strings.TrimSpace(section)
	if section == "" {
		return sections
	}
	return append(sections, section)
}

func firstNonEmptyPrompt(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isMainAgentPrompt(agentValue *protocol.Agent, defaultAgentID string) bool {
	if agentValue == nil {
		return false
	}
	return agentValue.IsMain || strings.TrimSpace(agentValue.AgentID) == strings.TrimSpace(defaultAgentID)
}

func buildRuntimeScopeSection(ctx context.Context) string {
	principal := authctx.PrincipalFromContext(ctx)
	state, hasState := authctx.StateFromContext(ctx)
	userID, hasUserID := authctx.CurrentUserID(ctx)

	lines := []string{"## Runtime Scope"}
	switch {
	case hasUserID:
		lines = append(lines,
			"Mode: multi-user user scope",
			"Current user_id: "+userID,
		)
		if principal != nil && strings.TrimSpace(principal.Username) != "" {
			lines = append(lines, "Current username: "+strings.TrimSpace(principal.Username))
		}
		lines = append(lines, "Scope: this user only.")
	case hasState && state.AuthRequired:
		lines = append(lines, "Mode: authenticated system scope")
	default:
		lines = append(lines,
			"Mode: single-user system scope",
			"Current principal: "+authctx.SystemUserID,
		)
	}
	return strings.Join(lines, "\n")
}

func buildAgentProfileSection(agentValue *protocol.Agent, scope promptBuildScope) string {
	if agentValue == nil {
		return ""
	}
	agentID := strings.TrimSpace(agentValue.AgentID)
	identityName := strings.TrimSpace(agentValue.Name)
	if identityName == "" {
		identityName = agentID
	}

	lines := []string{"## Agent Identity"}
	if identityName != "" || agentID != "" {
		lines = append(lines, fmt.Sprintf("Identity: %s (%s)", identityName, agentID))
	}
	if strings.TrimSpace(scope.workspacePath) != "" {
		lines = append(lines, "WORKING DIRECTORY: "+strings.TrimSpace(scope.workspacePath))
	}

	description := strings.TrimSpace(agentValue.Description)
	vibeTags := compactStringValues(agentValue.VibeTags)
	if description == "" && len(vibeTags) == 0 {
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "", "## Agent Profile")
	if description != "" {
		lines = append(lines, "Description: "+description)
	}
	if len(vibeTags) > 0 {
		lines = append(lines, "Vibe Tags: "+strings.Join(vibeTags, ", "))
	}
	return strings.Join(lines, "\n")
}

func compactStringValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func loadWorkspacePromptSections(scope promptBuildScope) ([]string, error) {
	if strings.TrimSpace(scope.workspacePath) == "" {
		return nil, nil
	}
	files := scope.workspacePromptFiles()
	sections := make([]string, 0, len(files))
	for _, fileName := range files {
		content, err := readOptionalWorkspacePromptFile(scope.workspacePath, fileName)
		if err != nil {
			return nil, err
		}
		sections = appendPromptSection(sections, content)
	}
	return sections, nil
}

func readOptionalWorkspacePromptFile(workspacePath string, fileName string) (string, error) {
	targetPath := filepath.Join(strings.TrimSpace(workspacePath), fileName)
	content, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

// BuildUserMessageSuffix 构建追加到最后一条用户消息后的动态上下文。
func (b *promptBuilder) BuildUserMessageSuffix(ctx context.Context, agentValue *protocol.Agent, emotionContextID string) string {
	workspacePath := ""
	if agentValue != nil {
		scope := b.newBuildScope(agentValue)
		workspacePath = scope.workspacePath
	}
	emotionView := LoadRuntimeEmotionView(workspacePath, emotionContextID, time.Now())
	sections := make([]string, 0, 2)
	sections = appendPromptSection(sections, b.buildRuntimeDateSection())
	sections = appendPromptSection(sections, buildRuntimeEmotionSection(agentValue, emotionView))
	if len(sections) == 0 {
		return ""
	}
	return strings.Join([]string{
		"<nexus_runtime_context>",
		strings.Join(sections, "\n\n"),
		"</nexus_runtime_context>",
	}, "\n")
}

func (b *promptBuilder) buildRuntimeDateSection() string {
	timezoneName := strings.TrimSpace(b.config.DefaultTimezone)
	if timezoneName == "" {
		timezoneName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		location = time.Local
		timezoneName = location.String()
		if strings.TrimSpace(timezoneName) == "" {
			timezoneName = "Local"
		}
	}
	now := time.Now().In(location)
	_, offsetSeconds := now.Zone()
	return strings.Join([]string{
		"## Date Awareness",
		fmt.Sprintf("Authoritative local time: %s (%s, %s, %s)", now.Format("2006-01-02 15:04:05"), now.Format("Monday"), timezoneName, formatUTCOffset(offsetSeconds)),
		"Relative date rule: interpret today, yesterday, tomorrow, this year, latest, recent, and equivalent phrases in the user's language from the time above. Do not guess or hardcode old years.",
	}, "\n")
}

func buildRuntimeEmotionSection(agentValue *protocol.Agent, view RuntimeEmotionView) string {
	name := strings.TrimSpace(agentValueName(agentValue))
	if name == "" {
		name = "Nexus"
	}
	lines := []string{
		"## Emotion State",
		fmt.Sprintf("Base: %s (energy %d/10, valence %d/10) - %s", view.Base.Mood, view.Base.Energy, view.Base.Valence, view.Base.Description),
	}
	if view.Context != nil {
		lines = append(lines, fmt.Sprintf("Context: %s (valence %d/10) - %s", view.Context.Mood, view.Context.Valence, view.Context.Trigger))
	}
	lines = append(lines,
		fmt.Sprintf("Composite: %s (energy %d/10, valence %d/10) - %s", view.Composite.Mood, view.Composite.Energy, view.Composite.Valence, view.Composite.Description),
		fmt.Sprintf("Fatigue: %s (%d/100)", view.Fatigue.Status, view.Fatigue.Level),
	)
	return strings.Join(lines, "\n")
}

func agentValueName(agentValue *protocol.Agent) string {
	if agentValue == nil {
		return ""
	}
	if name := strings.TrimSpace(agentValue.Name); name != "" {
		return name
	}
	return strings.TrimSpace(agentValue.AgentID)
}

func formatUTCOffset(offsetSeconds int) string {
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("UTC%s%02d:%02d", sign, hours, minutes)
}
