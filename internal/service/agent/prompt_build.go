// INPUT: Agent 配置、workspace 提示文件与运行时身份。
// OUTPUT: Agent 基础提示、身份画像与 workspace 指令。
// POS: Agent 服务的运行时 prompt 装配入口。
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
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var defaultWorkspacePromptFiles = []string{
	"AGENTS.md",
	"USER.md",
	"SOUL.md",
	"TOOLS.md",
}

var mainAgentWorkspacePromptFiles = []string{
	"USER.md",
}

type promptBuilder struct {
	config config.Config
}

type promptBuildScope struct {
	isMainAgent   bool
	ownerUserID   string
	workspacePath string
	workspaceRoot string
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
	for _, section := range buildAgentProfileSections(agentValue, scope) {
		sections = appendPromptSection(sections, section)
	}

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
		ownerUserID:   strings.TrimSpace(agentValue.OwnerUserID),
		workspacePath: workspacePath,
		workspaceRoot: strings.TrimSpace(b.config.WorkspacePath),
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
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
	lines = append(lines,
		"Temporary files: use $TMPDIR for private data; /tmp is a shared compatibility directory and must not contain secrets.",
	)
	return strings.Join(lines, "\n")
}

func buildAgentProfileSections(agentValue *protocol.Agent, scope promptBuildScope) []string {
	if agentValue == nil {
		return nil
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
	sections := []string{strings.Join(lines, "\n")}

	description := strings.TrimSpace(agentValue.Description)
	vibeTags := compactStringValues(agentValue.VibeTags)
	if description == "" && len(vibeTags) == 0 {
		return sections
	}

	lines = []string{"## Agent Profile"}
	if description != "" {
		lines = append(lines, "Description: "+description)
	}
	if len(vibeTags) > 0 {
		lines = append(lines, "Vibe Tags: "+strings.Join(vibeTags, ", "))
	}
	return append(sections, strings.Join(lines, "\n"))
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
		content, err := readOptionalWorkspacePromptFile(scope, fileName)
		if err != nil {
			return nil, err
		}
		sections = appendPromptSection(sections, formatWorkspacePromptSection(fileName, content))
	}
	return sections, nil
}

func readOptionalWorkspacePromptFile(
	scope promptBuildScope,
	fileName string,
) (string, error) {
	root, err := openPromptWorkspace(scope)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer root.Close()
	content, err := root.ReadFile(filepath.ToSlash(fileName))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func openPromptWorkspace(scope promptBuildScope) (*confinedfs.Root, error) {
	if strings.TrimSpace(scope.ownerUserID) != "" {
		return workspacestore.New(scope.workspaceRoot).OpenOwnerWorkspacePath(
			scope.ownerUserID,
			scope.workspacePath,
			false,
		)
	}
	return confinedfs.Open(strings.TrimSpace(scope.workspacePath))
}

// formatWorkspacePromptSection 在运行时提示词边界注入来源文件名，避免污染用户实际模板。
func formatWorkspacePromptSection(fileName string, content string) string {
	fileName = strings.TrimSpace(fileName)
	content = strings.TrimSpace(content)
	if fileName == "" || content == "" {
		return content
	}
	heading := "# " + fileName
	if content == heading || strings.HasPrefix(content, heading+"\n") {
		return content
	}
	return heading + "\n\n" + content
}

// BuildUserMessageSuffix 构建追加到最后一条用户消息后的动态上下文。
func (b *promptBuilder) BuildUserMessageSuffix(ctx context.Context, agentValue *protocol.Agent, emotionContextID string) string {
	scope := promptBuildScope{}
	if agentValue != nil {
		scope = b.newBuildScope(agentValue)
	}
	// 时间不再由本层注入：runtime（nexus-agent-sdk-go）的基础提示已含权威时间，
	// 且秒级时间戳会污染 prompt 前缀缓存。此处只保留情绪态。
	emotionView := loadRuntimeEmotionViewForScope(scope, emotionContextID, time.Now())
	sections := make([]string, 0, 1)
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

func loadRuntimeEmotionViewForScope(
	scope promptBuildScope,
	contextID string,
	now time.Time,
) RuntimeEmotionView {
	state := defaultRuntimeEmotionState(now)
	if strings.TrimSpace(scope.workspacePath) != "" {
		root, err := openPromptWorkspace(scope)
		if err == nil {
			state = loadRuntimeEmotionStateAt(root, now)
			_ = root.Close()
		}
	}
	return buildRuntimeEmotionView(scope.workspacePath, state, contextID, now)
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
