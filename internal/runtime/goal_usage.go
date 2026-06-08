package runtime

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// GoalUsageFromTokenUsage 把 SDK usage 转成 Goal accounting 口径。
func GoalUsageFromTokenUsage(usage sdkprotocol.TokenUsage) protocol.GoalUsage {
	return protocol.GoalUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		ReasoningTokens:          usage.ReasoningTokens,
		TotalTokens:              usage.TotalTokens,
	}
}

// GoalUsageFromRaw 从动态 usage JSON 提取 Goal accounting usage。
func GoalUsageFromRaw(raw any) (protocol.GoalUsage, bool) {
	usage, ok := sdkprotocol.ParseTokenUsage(raw)
	if !ok {
		return protocol.GoalUsage{}, false
	}
	return GoalUsageFromTokenUsage(usage), true
}

// ResultUsageLimitReached 判断 result 是否明确表示账号/计划 usage limit，而不是普通 token/context limit。
func ResultUsageLimitReached(result *sdkprotocol.ResultMessage) (bool, string) {
	if result == nil {
		return false, ""
	}
	candidates := []string{
		result.Subtype,
		result.TerminalReason,
		result.Result,
		fmt.Sprint(result.StopReason),
	}
	candidates = append(candidates, result.Errors...)
	candidates = append(candidates, usageLimitCandidateStrings(result.Additional)...)
	for _, candidate := range candidates {
		if textIndicatesUsageLimit(candidate) {
			return true, firstUsageLimitReason(result, candidate)
		}
	}
	return false, ""
}

func firstUsageLimitReason(result *sdkprotocol.ResultMessage, fallback string) string {
	for _, candidate := range []string{result.Result, fallback, result.TerminalReason} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "Runtime usage limit reached"
}

func usageLimitCandidateStrings(payload map[string]any) []string {
	if len(payload) == 0 {
		return nil
	}
	candidates := make([]string, 0, 12)
	var visit func(any, int)
	visit = func(value any, depth int) {
		if depth > 3 || value == nil {
			return
		}
		switch typed := value.(type) {
		case string:
			candidates = append(candidates, typed)
		case map[string]any:
			for _, key := range []string{
				"error_type",
				"type",
				"code",
				"error_code",
				"category",
				"message",
				"reason",
				"terminal_reason",
				"rate_limit_reached_type",
			} {
				visit(typed[key], depth+1)
			}
			visit(typed["error"], depth+1)
			visit(typed["details"], depth+1)
		case []any:
			for _, item := range typed {
				visit(item, depth+1)
			}
		case []string:
			candidates = append(candidates, typed...)
		}
	}
	visit(payload, 0)
	return candidates
}

func textIndicatesUsageLimit(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "usage_limit_reached", "usage_limit_exceeded", "usage_not_included":
		return true
	}
	compact := strings.NewReplacer("_", "", "-", "", " ", "", ".", "", "'", "").Replace(normalized)
	switch compact {
	case "usagelimitreached", "usagelimitexceeded", "usagenotincluded",
		"workspacememberusagelimitreached", "workspaceownerusagelimitreached":
		return true
	}
	return strings.Contains(normalized, "hit your usage limit") ||
		strings.Contains(normalized, "reached your usage limit") ||
		strings.Contains(normalized, "usage limit has been reached")
}
