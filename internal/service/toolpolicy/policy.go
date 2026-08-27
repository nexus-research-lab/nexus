// INPUT: 托管 Skill、nexus.command 与工具标识。
// OUTPUT: Nexus 内置能力的精确审批和允许工具集合。
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
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var managedVisualizeAllowedTools = []string{
	"mcp__nexus__show_widget",
	"show_widget",
}

var managedVisualizeToolNames = map[string]struct{}{
	"show_widget":             {},
	"mcp__nexus__show_widget": {},
	"nexus__show_widget":      {},
	"nexus.show_widget":       {},
	"nexus/show_widget":       {},
}

var managedImagegenAllowedTools = []string{
	"nexus_imagegen",
	"mcp__nexus__generate_image",
	"mcp__nexus__edit_image",
	"generate_image",
	"edit_image",
}

var managedMainThreadAllowedTools = []string{
	"Agent",
	"Skill",
}

var managedRuntimeCommandAllowedTools = []string{
	"mcp__nexus__command",
}

var managedRuntimeCommandToolNames = map[string]struct{}{
	"mcp__nexus__command": {},
	"nexus__command":      {},
	"nexus.command":       {},
	"nexus/command":       {},
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

var managedToolFamilyLeaves = map[string]map[string]struct{}{
	"nexus_communication": {
		"list_targets": {}, "send_message": {},
	},
	"nexus_visualize": {"show_widget": {}},
	"nexus_imagegen":  {"generate_image": {}, "edit_image": {}},
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
	leaves := managedToolFamilyLeaves[pair.approved]
	leaf := toolNameLeaf(pair.actual)
	if _, ok := leaves[leaf]; !ok {
		return false
	}
	if pair.actual == leaf {
		return true
	}
	for _, prefix := range []string{"mcp__nexus__", "nexus__", "nexus.", "nexus/"} {
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

// IsManagedSemanticSkillRequest 判断 Skill 调用是否只是在加载受管的
// Goal/Execution 语义 Skill。具体副作用仍只能走 round-scoped nexus.command。
func IsManagedSemanticSkillRequest(toolName string, input map[string]any) bool {
	if !MatchesItem(toolName, "Skill") {
		return false
	}
	for _, key := range []string{"name", "skill", "skill_name", "skillName"} {
		if agentsvc.IsManagedSemanticSkillName(stringInput(input, key)) {
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

// IsManagedRuntimeCommandTool 判断请求是否命中 round-scoped Nexus command 工具。
func IsManagedRuntimeCommandTool(toolName string) bool {
	_, ok := managedRuntimeCommandToolNames[strings.TrimSpace(toolName)]
	return ok
}

// WithManagedRuntimeAutoApproval 放行内置 Goal/Execution 语义 Skill、结构化
// Nexus command 与只在沙箱前端生效的生成式 UI 工具。
func WithManagedRuntimeAutoApproval(handler sdkpermission.Handler) sdkpermission.Handler {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if IsManagedSemanticSkillRequest(request.ToolName, request.Input) ||
			IsManagedRuntimeCommandTool(request.ToolName) ||
			IsManagedVisualizeTool(request.ToolName) {
			return sdkpermission.Allow(cloneInput(request.Input), nil), nil
		}
		return handler(ctx, request)
	}
}

// RuntimeCLIInvocation 是历史轨迹中经过严格 shell 子集解析的命令身份。
type RuntimeCLIInvocation struct {
	Domain string
	Action string
}

// NexusRuntimeCLIInvocation 只为旧 Runtime Graph 返回历史 CLI 的 domain/action。
// 它不参与当前工具审批，也不恢复已删除的 CLI transport。
func NexusRuntimeCLIInvocation(request sdkpermission.Request) (RuntimeCLIInvocation, bool) {
	invalid := RuntimeCLIInvocation{}
	rawCommand, ok := request.Input["command"].(string)
	if !ok || rawCommand == "" || strings.ContainsAny(rawCommand, "\n\r\x00") {
		return invalid, false
	}
	command := strings.Trim(rawCommand, " \t")
	if command == "" {
		return invalid, false
	}
	commandToken := ""
	managedInputToken := ""
	switch {
	case MatchesItem(request.ToolName, "Bash"):
		commandToken = `"${NEXUS_COMMAND_PATH}"`
		managedInputToken = `"${NEXUS_COMMAND_INPUT_PATH}"`
	case MatchesItem(request.ToolName, "PowerShell"):
		commandToken = `& "${env:NEXUS_COMMAND_PATH}"`
		managedInputToken = `"${env:NEXUS_COMMAND_INPUT_PATH}"`
	default:
		return invalid, false
	}
	if !strings.HasPrefix(command, commandToken) ||
		len(command) == len(commandToken) || !runtimeCommandShellWhitespace(command[len(commandToken)]) {
		return invalid, false
	}
	arguments, ok := parseRuntimeCommandShellArguments(command[len(commandToken):], managedInputToken)
	if !ok || len(arguments) < 3 || arguments[0] != "--json" {
		return invalid, false
	}
	invocation := RuntimeCLIInvocation{Domain: arguments[1], Action: arguments[2]}
	switch invocation.Domain {
	case "automation":
		if !validAutomationCLIArguments(invocation.Action, arguments[3:]) {
			return invalid, false
		}
	case "goal", "execution":
		if !validSemanticCLIArguments(invocation.Domain, invocation.Action, arguments[3:]) {
			return invalid, false
		}
	default:
		return invalid, false
	}
	return invocation, true
}

func validAutomationCLIArguments(action string, arguments []string) bool {
	if action == "contract" {
		return len(arguments) == 0
	}
	if action != "inspect" && action != "plan" && action != "apply" {
		return false
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
	for index := 0; index < len(arguments); index += 2 {
		if index+1 >= len(arguments) || !allowed[arguments[index]] || seen[arguments[index]] {
			return false
		}
		seen[arguments[index]] = true
		if arguments[index] == "--input-file" && arguments[index+1] != runtimeCommandShellInputPathToken {
			return false
		}
	}
	if !seen["--operation"] || seen["--input"] && seen["--input-file"] {
		return false
	}
	return true
}

func validSemanticCLIArguments(domain string, action string, arguments []string) bool {
	switch action {
	case "contract":
		return validRuntimeCLIFlags(arguments, map[string]bool{"--operation": true}, nil)
	case "inspect":
		if domain == command.DomainExecution {
			return validRuntimeCLIFlags(arguments, map[string]bool{"--execution-id": true}, nil)
		}
		return len(arguments) == 0
	case "invoke":
		return validRuntimeCLIFlags(arguments, map[string]bool{
			"--operation": true, "--request-id": true,
		}, map[string]bool{"--operation": true, "--request-id": true})
	default:
		return false
	}
}

func validRuntimeCLIFlags(arguments []string, allowed map[string]bool, required map[string]bool) bool {
	seen := make(map[string]bool, len(allowed))
	for index := 0; index < len(arguments); index += 2 {
		if index+1 >= len(arguments) || !allowed[arguments[index]] || seen[arguments[index]] {
			return false
		}
		seen[arguments[index]] = true
	}
	for flag := range required {
		if !seen[flag] {
			return false
		}
	}
	return true
}

const runtimeCommandShellInputPathToken = "\x00nexus-command-input-path\x00"

func parseRuntimeCommandShellArguments(command string, managedInput string) ([]string, bool) {
	arguments := make([]string, 0, 10)
	for index := 0; index < len(command); {
		for index < len(command) && runtimeCommandShellWhitespace(command[index]) {
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
				value = runtimeCommandShellInputPathToken
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
			for index < len(command) && !runtimeCommandShellWhitespace(command[index]) {
				if !runtimeCommandShellSafeByte(command[index]) {
					return nil, false
				}
				index++
			}
			value = command[start:index]
		}
		if value == "" || index < len(command) && !runtimeCommandShellWhitespace(command[index]) {
			// 拒绝 shell 的 quoted/unquoted token 拼接；即使结果 argv 相同，也不把
			// 更大的 shell 语法面纳入自动审批边界。
			return nil, false
		}
		arguments = append(arguments, value)
	}
	return arguments, len(arguments) > 0
}

func runtimeCommandShellWhitespace(value byte) bool {
	return value == ' ' || value == '\t'
}

func runtimeCommandShellSafeByte(value byte) bool {
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
				"Nexus control-plane CLI is unavailable in this runtime context; use round-scoped Nexus CLI and Skills",
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

// WithManagedImagegenAllowedTools 预授权图片生成 MCP 工具，保留用户原有工具设置。
func WithManagedImagegenAllowedTools(tools []string) []string {
	approved := NormalizeSet(tools)
	if len(approved) == 0 {
		return tools
	}
	if !Contains(approved, "mcp__nexus__generate_image") &&
		!Contains(approved, "mcp__nexus__edit_image") {
		return tools
	}
	return appendDistinctTools(tools, managedImagegenAllowedTools...)
}

// WithManagedRuntimeAllowedTools 追加内置 Skill、结构化 command 与 UI 能力的必要白名单。
func WithManagedRuntimeAllowedTools(tools []string, imagegenDefaultEnabled bool) []string {
	result := tools
	if len(NormalizeSet(result)) == 0 {
		return result
	}
	result = appendDistinctTools(result, managedMainThreadAllowedTools...)
	result = appendDistinctTools(result, managedRuntimeCommandAllowedTools...)
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
