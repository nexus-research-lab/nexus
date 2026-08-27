// INPUT: 当前偏好与是否存在有效图片模型选择。
// OUTPUT: 只增删规范 nexus_imagegen 名称的默认 Agent tool 列表。
// POS: Web 设置与对话配置共用的图片工具默认值投影。
package preferences

import "strings"

// ReconcileImagegenDefaultTool 让显式默认工具列表与图片模型可用性保持一致。
func ReconcileImagegenDefaultTool(prefs Preferences, enabled bool) (Preferences, bool) {
	tools, changed := normalizeImagegenDefaultTool(prefs.DefaultAgentOptions.AllowedTools, enabled)
	if !changed {
		return prefs, false
	}
	prefs.DefaultAgentOptions.AllowedTools = tools
	return prefs, true
}

func normalizeImagegenDefaultTool(tools []string, enabled bool) ([]string, bool) {
	result := make([]string, 0, len(tools)+1)
	hasImagegen := false
	hasExplicitTool := false
	changed := false
	for _, toolName := range tools {
		value := strings.TrimSpace(toolName)
		if value == "" {
			changed = true
			continue
		}
		hasExplicitTool = true
		if isImagegenToolName(value) {
			if !enabled {
				changed = true
				continue
			}
			if hasImagegen || value != "nexus_imagegen" {
				changed = true
			}
			if !hasImagegen {
				result = append(result, "nexus_imagegen")
				hasImagegen = true
			}
			continue
		}
		result = append(result, value)
	}
	if !hasExplicitTool {
		return result, changed
	}
	if enabled && !hasImagegen {
		result = append(result, "nexus_imagegen")
		changed = true
	}
	return result, changed
}

func isImagegenToolName(value string) bool {
	// 旧 server 包装名只用于清洗已持久化的偏好。
	switch strings.TrimSpace(value) {
	case "nexus_imagegen",
		"generate_image",
		"edit_image",
		"mcp__nexus__generate_image",
		"mcp__nexus__edit_image",
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image":
		return true
	default:
		return false
	}
}
