// INPUT: 模型 ID 与已合并的推理能力。
// OUTPUT: 该模型在 Session 设置里可选的思考强度档位（由低到高）。
// POS: 宿主侧强度目录；只决定展示哪些档位，runtime 仍按自身模型卡归一化实际请求值。
package provider

import (
	"strconv"
	"strings"
)

var (
	claudeEffortLevels   = []string{"low", "medium", "high", "max"}
	openAIEffortLevels   = []string{"low", "medium", "high"}
	deepSeekEffortLevels = []string{"low", "high", "max"}
	// GLM-5.3 强制思考，档位与 SDK 模型卡一致；5.1/5.2 尚无已验证的强度策略。
	glmEffortLevels = []string{"low", "high", "max"}
)

// modelEffortLevels 按模型家族返回可选强度；未知模型不开放，避免展示无效档位。
func modelEffortLevels(modelID string, reasoning *bool) []string {
	if reasoning != nil && !*reasoning {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.Contains(normalized, "deepseek-v4"), strings.Contains(normalized, "deepseek-flash"):
		return deepSeekEffortLevels
	case strings.Contains(normalized, "glm-5.3"):
		return glmEffortLevels
	case claudeSupportsEffort(normalized):
		return claudeEffortLevels
	case strings.HasPrefix(normalized, "gpt-5"),
		strings.HasPrefix(normalized, "o1"),
		strings.HasPrefix(normalized, "o3"),
		strings.HasPrefix(normalized, "o4"):
		return openAIEffortLevels
	}
	return nil
}

// claudeSupportsEffort 只认 Opus/Sonnet 4.6 及以后；Haiku 与旧版本不接受 effort。
func claudeSupportsEffort(normalized string) bool {
	for _, family := range []string{"opus-", "sonnet-"} {
		index := strings.Index(normalized, family)
		if index < 0 {
			continue
		}
		major, minor := claudeModelVersion(normalized[index+len(family):])
		return major > 4 || (major == 4 && minor >= 6)
	}
	return false
}

// claudeModelVersion 解析 "4-6"、"5"、"5-5[1m]"、"4-5-20250929" 这类版本尾；日期段不算小版本。
func claudeModelVersion(rest string) (int, int) {
	parts := strings.FieldsFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if len(parts) == 0 || !strings.HasPrefix(rest, parts[0]) {
		return 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 && len(parts[1]) <= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}
