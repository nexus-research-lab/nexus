// INPUT: managed skill 名称与 runtime 工具标识。
// OUTPUT: Goal 等托管能力的允许工具集合。
// POS: Agent runtime 工具策略的统一投影。
package toolpolicy

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const managedGoalSkillName = "goal-manager"

var managedGoalTools = []string{
	"nexus_goal",
	"get_goal",
	"create_goal",
	"retarget_goal",
	"audit_objective_alignment",
	"update_goal",
}

var managedGoalAllowedTools = []string{
	"nexus_goal",
	"mcp__nexus_goal__get_goal",
	"mcp__nexus_goal__create_goal",
	"mcp__nexus_goal__retarget_goal",
	"mcp__nexus_goal__audit_objective_alignment",
	"mcp__nexus_goal__update_goal",
	"get_goal",
	"create_goal",
	"retarget_goal",
	"audit_objective_alignment",
	"update_goal",
	"Skill",
}

var managedExecutionTools = []string{
	"get_execution",
	"prepare_plan_execution",
	"plan_execution",
	"abandon_execution",
	"assign_work",
	"submit_work",
	"review_work",
	"block_work",
	"resume_work",
	"take_over_work",
	"audit_execution_alignment",
	"promote_execution_to_goal",
}

var managedExecutionAllowedTools = []string{
	"nexus_execution",
	"mcp__nexus_execution__get_execution",
	"mcp__nexus_execution__prepare_plan_execution",
	"mcp__nexus_execution__plan_execution",
	"mcp__nexus_execution__abandon_execution",
	"mcp__nexus_execution__assign_work",
	"mcp__nexus_execution__submit_work",
	"mcp__nexus_execution__review_work",
	"mcp__nexus_execution__block_work",
	"mcp__nexus_execution__resume_work",
	"mcp__nexus_execution__take_over_work",
	"mcp__nexus_execution__audit_execution_alignment",
	"mcp__nexus_execution__promote_execution_to_goal",
	"get_execution",
	"prepare_plan_execution",
	"plan_execution",
	"abandon_execution",
	"assign_work",
	"submit_work",
	"review_work",
	"block_work",
	"resume_work",
	"take_over_work",
	"audit_execution_alignment",
	"promote_execution_to_goal",
}

var managedVisualizeAllowedTools = []string{
	"nexus_visualize",
	"mcp__nexus_visualize__show_widget",
	"show_widget",
}

var managedVisualizeToolNames = map[string]struct{}{
	"show_widget":                       {},
	"mcp__nexus_visualize__show_widget": {},
	"nexus_visualize__show_widget":      {},
	"nexus_visualize.show_widget":       {},
	"nexus_visualize/show_widget":       {},
}

var managedImagegenAllowedTools = []string{
	"nexus_imagegen",
	"mcp__nexus_imagegen__generate_image",
	"mcp__nexus_imagegen__edit_image",
	"generate_image",
	"edit_image",
}

var managedMainThreadAllowedTools = []string{
	"Agent",
}

// NormalizeSet 把工具名列表归一成集合；nil/空列表表示没有显式策略。
func NormalizeSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// Contains 判断工具名是否命中集合，支持 SDK/MCP 包装后的常见命名。
func Contains(approved map[string]struct{}, toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return false
	}
	if _, ok := approved[toolName]; ok {
		return true
	}
	for item := range approved {
		if MatchesItem(toolName, item) {
			return true
		}
	}
	return false
}

// MatchesItem 处理 mcp__server__tool / server.tool / server/tool 这类包装名。
func MatchesItem(toolName string, approved string) bool {
	pair := toolNamePair{
		actual:   strings.TrimSpace(toolName),
		approved: strings.TrimSpace(approved),
	}
	if pair.actual == "" || pair.approved == "" {
		return false
	}
	for _, matcher := range toolNameMatchers {
		if matcher(pair) {
			return true
		}
	}
	return false
}

type toolNamePair struct {
	actual   string
	approved string
}

type toolNameMatcher func(toolNamePair) bool

var toolNameMatchers = []toolNameMatcher{
	matchesWrappedToolName,
	matchesCanonicalToolName,
	matchesCanonicalToolLeaf,
	matchesKnownToolAlias,
	matchesManagedToolFamily,
}

var managedToolFamilyPrefixes = map[string][]string{
	"nexus_automation": {"mcp__nexus_automation__", "nexus_automation__", "nexus_automation."},
	"nexus_execution":  {"mcp__nexus_execution__", "nexus_execution__", "nexus_execution."},
	"nexus_goal":       {"mcp__nexus_goal__", "nexus_goal__", "nexus_goal."},
	"nexus_room":       {"mcp__nexus_room__", "nexus_room__", "nexus_room."},
	"nexus_visualize":  {"mcp__nexus_visualize__", "nexus_visualize__", "nexus_visualize."},
	"nexus_imagegen":   {"mcp__nexus_imagegen__", "nexus_imagegen__", "nexus_imagegen."},
}

func matchesWrappedToolName(pair toolNamePair) bool {
	for _, separator := range []string{"__", ".", "/"} {
		if strings.HasSuffix(pair.actual, separator+pair.approved) {
			return true
		}
	}
	return false
}

func matchesCanonicalToolName(pair toolNamePair) bool {
	return canonicalToolName(pair.actual) == canonicalToolName(pair.approved)
}

func matchesCanonicalToolLeaf(pair toolNamePair) bool {
	return canonicalToolName(toolNameLeaf(pair.actual)) == canonicalToolName(pair.approved)
}

func matchesKnownToolAlias(pair toolNamePair) bool {
	return matchesKnownAlias(pair.actual, pair.approved)
}

func matchesManagedToolFamily(pair toolNamePair) bool {
	for _, prefix := range managedToolFamilyPrefixes[pair.approved] {
		if strings.HasPrefix(pair.actual, prefix) {
			return true
		}
	}
	return false
}

func matchesKnownAlias(toolName string, approved string) bool {
	approvedCanonical := canonicalToolName(approved)
	toolCanonical := canonicalToolName(toolNameLeaf(toolName))
	switch approvedCanonical {
	case "websearch":
		return toolCanonical == "search" || strings.HasSuffix(toolCanonical, "websearch")
	case "webfetch":
		return toolCanonical == "fetch" || strings.HasSuffix(toolCanonical, "webfetch")
	default:
		return false
	}
}

// IsManagedGoalTool 判断请求是否命中 Nexus 托管的 Goal MCP 工具。
func IsManagedGoalTool(toolName string) bool {
	for _, item := range managedGoalTools {
		if MatchesItem(toolName, item) {
			return true
		}
	}
	return false
}

// IsManagedGoalSkillRequest 判断 Skill 调用是否只是在加载内置 goal-manager。
func IsManagedGoalSkillRequest(toolName string, input map[string]any) bool {
	if !MatchesItem(toolName, "Skill") {
		return false
	}
	for _, key := range []string{"name", "skill", "skill_name", "skillName"} {
		if canonicalToolName(stringInput(input, key)) == canonicalToolName(managedGoalSkillName) {
			return true
		}
	}
	return false
}

// IsManagedGoalPermission 判断权限请求是否属于产品托管 Goal 能力。
func IsManagedGoalPermission(toolName string, input map[string]any) bool {
	return IsManagedGoalTool(toolName) || IsManagedGoalSkillRequest(toolName, input)
}

// IsManagedExecutionTool 判断请求是否命中 Nexus 托管的 Execution MCP 工具。
func IsManagedExecutionTool(toolName string) bool {
	for _, item := range managedExecutionTools {
		if MatchesItem(toolName, item) {
			return true
		}
	}
	return false
}

// IsManagedVisualizeTool 判断请求是否命中 Nexus 托管的生成式 UI 工具。
func IsManagedVisualizeTool(toolName string) bool {
	_, ok := managedVisualizeToolNames[strings.TrimSpace(toolName)]
	return ok
}

// WithManagedGoalAutoApproval 让隐藏续跑和模型自启动 Goal 时不被内置 Goal 工具确认卡住。
func WithManagedGoalAutoApproval(handler sdkpermission.Handler) sdkpermission.Handler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if IsManagedGoalPermission(request.ToolName, request.Input) {
			return sdkpermission.Allow(cloneInput(request.Input), nil), nil
		}
		return handler(ctx, request)
	}
}

// WithManagedRuntimeAutoApproval 放行 Nexus 托管语义工具与只在沙箱前端生效的
// 生成式 UI 工具，避免内部能力被通用权限卡中断。
func WithManagedRuntimeAutoApproval(handler sdkpermission.Handler) sdkpermission.Handler {
	handler = WithManagedGoalAutoApproval(handler)
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if IsManagedExecutionTool(request.ToolName) || IsManagedVisualizeTool(request.ToolName) {
			return sdkpermission.Allow(cloneInput(request.Input), nil), nil
		}
		return handler(ctx, request)
	}
}

// WithNexusRuntimeCLIAutoApproval 只放行一个没有 shell 组合符的 exact
// `nexus automation` 进程调用。查询/plan 由 round capability 收口，apply 还会在
// broker 内发起独立原生真人确认，因此这里不能成为领域写入授权。
func WithNexusRuntimeCLIAutoApproval(handler sdkpermission.Handler) sdkpermission.Handler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if IsNexusAutomationCLIRequest(request) {
			return sdkpermission.Allow(cloneInput(request.Input), nil), nil
		}
		return handler(ctx, request)
	}
}

// IsNexusAutomationCLIRequest 只识别受管 executable、固定子命令和无 shell
// 动态语义的单进程调用；任何其他 Bash/PowerShell 语法仍进入普通权限处理器。
func IsNexusAutomationCLIRequest(request sdkpermission.Request) bool {
	_, ok := NexusAutomationCLIAction(request)
	return ok
}

// NexusAutomationCLIAction 返回经过严格 shell 子集解析后的 Automation action。
// executable 和 input slot 都只能引用宿主注入的精确变量，不能换成同名程序或任意路径。
func NexusAutomationCLIAction(request sdkpermission.Request) (string, bool) {
	rawCommand, ok := request.Input["command"].(string)
	if !ok || rawCommand == "" || strings.ContainsAny(rawCommand, "\n\r\x00") {
		return "", false
	}
	command := strings.Trim(rawCommand, " \t")
	if command == "" {
		return "", false
	}
	commandToken := ""
	managedInputToken := ""
	switch {
	case MatchesItem(request.ToolName, "Bash"):
		commandToken = `"${NEXUS_COMMAND_PATH}"`
		managedInputToken = `"${NEXUS_AUTOMATION_INPUT_PATH}"`
	case MatchesItem(request.ToolName, "PowerShell"):
		commandToken = `& "${env:NEXUS_COMMAND_PATH}"`
		managedInputToken = `"${env:NEXUS_AUTOMATION_INPUT_PATH}"`
	default:
		return "", false
	}
	if !strings.HasPrefix(command, commandToken) ||
		len(command) == len(commandToken) || !automationShellWhitespace(command[len(commandToken)]) {
		return "", false
	}
	arguments, ok := parseAutomationShellArguments(command[len(commandToken):], managedInputToken)
	if !ok || len(arguments) < 3 || arguments[0] != "--json" || arguments[1] != "automation" {
		return "", false
	}
	action := arguments[2]
	if action == "contract" {
		return action, len(arguments) == 3
	}
	if action != "inspect" && action != "plan" && action != "apply" {
		return "", false
	}
	allowed := map[string]bool{
		"--operation":  true,
		"--input":      true,
		"--input-file": true,
	}
	if action == "apply" {
		allowed["--request-id"] = true
		allowed["--expected-revision"] = true
	}
	seen := make(map[string]bool, len(allowed))
	for index := 3; index < len(arguments); index += 2 {
		if index+1 >= len(arguments) || !allowed[arguments[index]] || seen[arguments[index]] {
			return "", false
		}
		seen[arguments[index]] = true
		if arguments[index] == "--input-file" && arguments[index+1] != automationShellInputPathToken {
			return "", false
		}
	}
	if !seen["--operation"] || seen["--input"] && seen["--input-file"] {
		return "", false
	}
	return action, true
}

const automationShellInputPathToken = "\x00nexus-automation-input-path\x00"

func parseAutomationShellArguments(command string, managedInput string) ([]string, bool) {
	arguments := make([]string, 0, 10)
	for index := 0; index < len(command); {
		for index < len(command) && automationShellWhitespace(command[index]) {
			index++
		}
		if index == len(command) {
			break
		}
		var value string
		switch command[index] {
		case '\'':
			end := strings.IndexByte(command[index+1:], '\'')
			if end < 0 {
				return nil, false
			}
			end += index + 1
			value = command[index+1 : end]
			index = end + 1
		case '"':
			if strings.HasPrefix(command[index:], managedInput) {
				value = automationShellInputPathToken
				index += len(managedInput)
				break
			}
			end := strings.IndexByte(command[index+1:], '"')
			if end < 0 {
				return nil, false
			}
			end += index + 1
			quoted := command[index+1 : end]
			if strings.ContainsAny(quoted, "$`\\") {
				return nil, false
			}
			value = quoted
			index = end + 1
		default:
			start := index
			for index < len(command) && !automationShellWhitespace(command[index]) {
				if !automationShellSafeByte(command[index]) {
					return nil, false
				}
				index++
			}
			value = command[start:index]
		}
		if value == "" || index < len(command) && !automationShellWhitespace(command[index]) {
			// 拒绝 shell 的 quoted/unquoted token 拼接；即使结果 argv 相同，也不把
			// 更大的 shell 语法面纳入自动审批边界。
			return nil, false
		}
		arguments = append(arguments, value)
	}
	return arguments, len(arguments) > 0
}

func automationShellWhitespace(value byte) bool {
	return value == ' ' || value == '\t'
}

func automationShellSafeByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		strings.ContainsRune("_-./:@%+=,", rune(value))
}

// WithMalformedInputDeny 检测工具输入 JSON 解析失败时拒绝执行，
// 将错误原因反馈给大模型使其可以重试或纠正，同时前端能看到出错工具调用。
func WithMalformedInputDeny(handler sdkpermission.Handler) sdkpermission.Handler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if rawParseError, ok := request.Input["_nexus_parse_error"]; ok {
			parseError, _ := rawParseError.(string)
			message := "工具输入 JSON 解析失败"
			if parseError != "" {
				message = fmt.Sprintf("工具输入 JSON 解析失败: %s", parseError)
			}
			if rawRawInput, ok := request.Input["_nexus_raw_input"]; ok {
				if rawInput, ok := rawRawInput.(string); ok && rawInput != "" {
					truncated := rawInput
					if len(truncated) > 200 {
						truncated = truncated[:200] + "..."
					}
					message = fmt.Sprintf("%s（原始输入: %s）", message, truncated)
				}
			}
			return sdkpermission.Deny(message, false), nil
		}
		return handler(ctx, request)
	}
}

// WithNexusControlPlaneDeny prevents a runtime without owner-main DM authority
// from reaching Nexus' host CLI through a shell request. The authoritative
// boundary is paired with removing the CLI path and owner scope from that
// runtime's environment; this guard also rejects explicit bypass attempts.
func WithNexusControlPlaneDeny(handler sdkpermission.Handler, denied bool) sdkpermission.Handler {
	if handler == nil || !denied {
		return handler
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if isNexusControlPlaneShellRequest(request) {
			return sdkpermission.Deny(
				"Nexus control-plane CLI is unavailable in this runtime context; use the scoped Nexus MCP tools",
				false,
			), nil
		}
		return handler(ctx, request)
	}
}

func isNexusControlPlaneShellRequest(request sdkpermission.Request) bool {
	if !MatchesItem(request.ToolName, "Bash") {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(stringInput(request.Input, "command")))
	if command == "" {
		return false
	}
	for _, marker := range []string{
		"nexusctl",
		"nexus_ctl",
		"nexus-ctl",
		"nexusctl_command_path",
		"cmd/nexusctl",
		"cmd\\nexusctl",
	} {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

// WithManagedGoalAllowedTools 预授权 Goal MCP 工具，保留用户原有工具设置。
func WithManagedGoalAllowedTools(tools []string) []string {
	if len(NormalizeSet(tools)) == 0 {
		return tools
	}
	return appendDistinctTools(tools, managedGoalAllowedTools...)
}

// WithManagedExecutionAllowedTools 预授权 Nexus 托管的语义编排工具。
func WithManagedExecutionAllowedTools(tools []string) []string {
	if len(NormalizeSet(tools)) == 0 {
		return tools
	}
	return appendDistinctTools(tools, managedExecutionAllowedTools...)
}

// WithManagedImagegenAllowedTools 预授权图片生成 MCP 工具，保留用户原有工具设置。
func WithManagedImagegenAllowedTools(tools []string) []string {
	approved := NormalizeSet(tools)
	if len(approved) == 0 {
		return tools
	}
	if !Contains(approved, "mcp__nexus_imagegen__generate_image") &&
		!Contains(approved, "mcp__nexus_imagegen__edit_image") {
		return tools
	}
	return appendDistinctTools(tools, managedImagegenAllowedTools...)
}

// WithManagedRuntimeAllowedTools 追加运行时内建 MCP 工具的必要白名单。
func WithManagedRuntimeAllowedTools(tools []string, imagegenDefaultEnabled bool) []string {
	result := WithManagedGoalAllowedTools(tools)
	if len(NormalizeSet(result)) == 0 {
		return result
	}
	result = WithManagedExecutionAllowedTools(result)
	result = appendDistinctTools(result, managedMainThreadAllowedTools...)
	result = appendDistinctTools(result, managedVisualizeAllowedTools...)
	if !imagegenDefaultEnabled {
		return withoutManagedImagegenAllowedTools(result)
	}
	result = appendDistinctTools(result, "nexus_imagegen")
	return WithManagedImagegenAllowedTools(result)
}

func withoutManagedImagegenAllowedTools(tools []string) []string {
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		if slices.Contains(managedImagegenAllowedTools, strings.TrimSpace(tool)) {
			continue
		}
		result = append(result, tool)
	}
	return result
}

func toolNameLeaf(toolName string) string {
	result := strings.TrimSpace(toolName)
	for _, separator := range []string{"__", ".", "/"} {
		if index := strings.LastIndex(result, separator); index >= 0 {
			result = result[index+len(separator):]
		}
	}
	return result
}

func canonicalToolName(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func stringInput(input map[string]any, key string) string {
	if len(input) == 0 {
		return ""
	}
	value, ok := input[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func cloneInput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	return maps.Clone(input)
}

func appendDistinctTools(base []string, extra ...string) []string {
	result := make([]string, 0, len(base)+len(extra))
	seen := make(map[string]struct{}, len(base)+len(extra))
	for _, tool := range slices.Concat(base, extra) {
		normalized := strings.TrimSpace(tool)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

// MergeSets 合并多个工具集合。
func MergeSets(sets ...map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for _, set := range sets {
		maps.Copy(result, set)
	}
	return result
}

// CopySet 复制工具集合。
func CopySet(items map[string]struct{}) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	return maps.Clone(items)
}
