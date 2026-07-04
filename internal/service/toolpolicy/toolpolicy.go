package toolpolicy

import (
	"context"
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
	"update_goal",
}

var managedGoalAllowedTools = []string{
	"nexus_goal",
	"mcp__nexus_goal__get_goal",
	"mcp__nexus_goal__create_goal",
	"mcp__nexus_goal__update_goal",
	"get_goal",
	"create_goal",
	"update_goal",
	"Skill",
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
	toolName = strings.TrimSpace(toolName)
	approved = strings.TrimSpace(approved)
	if toolName == "" || approved == "" {
		return false
	}
	if strings.HasSuffix(toolName, "__"+approved) ||
		strings.HasSuffix(toolName, "."+approved) ||
		strings.HasSuffix(toolName, "/"+approved) {
		return true
	}
	if canonicalToolName(toolName) == canonicalToolName(approved) {
		return true
	}
	if canonicalToolName(toolNameLeaf(toolName)) == canonicalToolName(approved) {
		return true
	}
	if matchesKnownAlias(toolName, approved) {
		return true
	}
	if approved == "nexus_automation" {
		return strings.HasPrefix(toolName, "mcp__nexus_automation__") ||
			strings.HasPrefix(toolName, "nexus_automation__") ||
			strings.HasPrefix(toolName, "nexus_automation.")
	}
	if approved == "nexus_goal" {
		return strings.HasPrefix(toolName, "mcp__nexus_goal__") ||
			strings.HasPrefix(toolName, "nexus_goal__") ||
			strings.HasPrefix(toolName, "nexus_goal.")
	}
	if approved == "nexus_room" {
		return strings.HasPrefix(toolName, "mcp__nexus_room__") ||
			strings.HasPrefix(toolName, "nexus_room__") ||
			strings.HasPrefix(toolName, "nexus_room.")
	}
	if approved == "nexus_imagegen" {
		return strings.HasPrefix(toolName, "mcp__nexus_imagegen__") ||
			strings.HasPrefix(toolName, "nexus_imagegen__") ||
			strings.HasPrefix(toolName, "nexus_imagegen.")
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

// WithManagedGoalAllowedTools 预授权 Goal MCP 工具，保留用户原有工具设置。
func WithManagedGoalAllowedTools(tools []string) []string {
	if len(NormalizeSet(tools)) == 0 {
		return tools
	}
	return appendDistinctTools(tools, managedGoalAllowedTools...)
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
	result = appendDistinctTools(result, managedMainThreadAllowedTools...)
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
