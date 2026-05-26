package toolpolicy

import (
	"strings"
	"unicode"
)

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

// MergeSets 合并多个工具集合。
func MergeSets(sets ...map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for _, set := range sets {
		for item := range set {
			result[item] = struct{}{}
		}
	}
	return result
}

// CopySet 复制工具集合。
func CopySet(items map[string]struct{}) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(items))
	for key := range items {
		result[key] = struct{}{}
	}
	return result
}
