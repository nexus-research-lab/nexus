// INPUT: Agent 配置、workspace managed skills 与运行时能力。
// OUTPUT: Agent system prompt、Goal 工具使用约束、创建前 readiness gate 与完成后的 result-first 交付契约。
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
	isMainAgent        bool
	ownerUserID        string
	workspacePath      string
	workspaceRoot      string
	skillNames         []string
	disabledSkillNames []string
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
	sections = appendPromptSection(sections, buildManagedSkillUsageSection(scope))

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
	disabledSkillNames := make(map[string]struct{}, len(agentValue.Options.DisabledSkillIDs))
	disabledSkillList := make([]string, 0, len(agentValue.Options.DisabledSkillIDs))
	for _, reference := range agentValue.Options.DisabledSkillIDs {
		name := canonicalPromptSkillName(reference)
		if name != "" {
			disabledSkillNames[strings.ToLower(name)] = struct{}{}
			disabledSkillList = append(disabledSkillList, name)
		}
	}
	skillNames := make([]string, 0, len(agentValue.Options.SkillIDs))
	for _, reference := range agentValue.Options.SkillIDs {
		name := canonicalPromptSkillName(reference)
		if name == "" {
			continue
		}
		if _, disabled := disabledSkillNames[strings.ToLower(name)]; disabled {
			continue
		}
		skillNames = append(skillNames, name)
	}
	return promptBuildScope{
		isMainAgent:        isMainAgentPrompt(agentValue, b.config.DefaultAgentID),
		ownerUserID:        strings.TrimSpace(agentValue.OwnerUserID),
		workspacePath:      workspacePath,
		workspaceRoot:      strings.TrimSpace(b.config.WorkspacePath),
		skillNames:         skillNames,
		disabledSkillNames: disabledSkillList,
	}
}

func canonicalPromptSkillName(reference string) string {
	name := strings.TrimSpace(reference)
	if externalName, ok := protocol.ParseExternalSkillReference(name); ok {
		name = externalName
	}
	return strings.TrimSpace(name)
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

func buildManagedSkillUsageSection(scope promptBuildScope) string {
	sections := []string{}
	if hasSelectedSkill(scope, "goal-manager") {
		sections = append(sections, strings.Join([]string{
			"## Goal Skill 使用要求",
			"- 用户明确要求启动、设定、继续或纠正当前会话 Goal 时，必须先使用 Skill 工具加载 goal-manager，再调用当前工具列表中可见的 Goal MCP 工具。",
			"- Nexus 中 Goal MCP 工具通常显示为 mcp__nexus_goal__get_goal、mcp__nexus_goal__create_goal、mcp__nexus_goal__retarget_goal、mcp__nexus_goal__audit_objective_alignment、mcp__nexus_goal__update_goal；如果运行时只暴露对应裸名，它们是同一组能力。",
			"- 不要使用 /goal 文本命令；Goal 的模型入口是 goal-manager + mcp__nexus_goal__* 工具，用户入口是界面的启动 Goal 按钮。",
			"- `create_goal` 只用于用户或系统/开发者明确要求持久 Goal 的显式路径；普通一次性请求、提醒和定时任务不要自动创建 Goal，也不要调用它。",
			"- 自适应 Goal 走 Execution 的 `promote_execution_to_goal`，不是猜测性调用 `create_goal`：当前 execution context 开放该 action 时，由你判断 objective 是否应跨当前执行边界持续。跨轮工作、外部等待、恢复成本、Room 依赖或 substantial complexity 都可作为理由；后端只校验权限、用户配置、冲突和状态一致性，不要求固定持久证据白名单。Plan、Room 或子智能体参与本身既不强制创建 Goal，也不禁止你结合任务事实选择 Goal。",
			"- 用户明确要求 Goal 只是创建的必要条件，不是立即创建指令。调用 create_goal 前，必须先从当前上下文确认 objective 已达到可执行状态：目标交付物，以及会实质改变结果的范围、对象、约束和验收标准等关键信息已经明确。",
			"- 若仍缺少会实质改变执行结果的信息，先向用户提问并等待回答；信息足够前禁止调用 create_goal，禁止先创建宽泛或占位 Goal 再补信息或 retarget。能从已有上下文可靠确定的信息不要重复询问。",
			"- 信息足够后，把已确认的关键要求合并成完整、具体的 objective，再创建 Goal 并按该 objective 执行。",
			"- 只有用户明确纠正当前 active Goal 的 objective 时才调用 retarget_goal；必须保留同一 Goal，绝不能先完成旧 Goal 再创建新 Goal。",
			"- token_budget 只有用户明确给出预算时才传；暂停、恢复、清理、预算限制和用量限制由用户或系统控制。",
			"- 只有 Goal 与 managed WorkGraph 已 confirmed 绑定时，完成前才必须用 audit_objective_alignment 对服务端给出的 objective 与 completion criteria 提交逐项证据；只有当前 objective revision、当前 round 的 aligned 审计可紧接着调用 update_goal(complete)。Goal-only 与 reserved Goal 不要求该审计；审计本身不改变 Goal 生命周期。",
			"- update_goal(complete) 只负责内部状态收口；工具成功后的最终回复才是用户交付面，必须独立、完整地呈现 objective 要求的成果。文本类交付直接给出完整正文；文件、实现、研究或外部操作类交付给出准确产物位置、核心结果和必要验证。",
			"- 最终回复以成果本身为重点，不得用“Goal 已完成”或简短总结代替结果，也不要让用户回看过程消息拼凑交付物；完成状态最多作为次要说明或省略。",
			"- 同一阻塞条件连续出现且无法推进时，才可标记 blocked。",
		}, "\n"))
	}
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func hasSelectedSkill(scope promptBuildScope, skillName string) bool {
	for _, disabled := range scope.disabledSkillNames {
		if strings.EqualFold(strings.TrimSpace(disabled), skillName) {
			return false
		}
	}
	for _, selected := range scope.skillNames {
		if strings.EqualFold(strings.TrimSpace(selected), skillName) {
			return true
		}
	}
	// 兼容旧 Agent：迁移完成前仍允许从 workspace 文件判断已部署状态。
	if strings.TrimSpace(scope.workspacePath) == "" {
		return false
	}
	root, err := openPromptWorkspace(scope)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Stat(filepath.ToSlash(filepath.Join(".agents", "skills", skillName, "SKILL.md")))
	return err == nil
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
