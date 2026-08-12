// INPUT: bridge provider usage 与动态 runtime usage JSON。
// OUTPUT: 校验 provider actual total 并保留 breakdown 的 Goal actual/budget 双口径 usage。
// POS: SDK token 协议到 Nexus Goal accounting 的转换边界。
package runtime

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// GoalUsageFromTokenUsage 把 SDK usage 转成 Goal accounting 口径。
func GoalUsageFromTokenUsage(usage sdkprotocol.TokenUsage) protocol.GoalUsage {
	goalUsage, _ := GoalUsageFromTokenUsageWithPresence(usage)
	return goalUsage
}

// GoalUsageFromTokenUsageWithPresence 转换 SDK usage，并区分“显式零用量”
// 与“结果没有 usage”。provider total 或 breakdown 字段即使全为 0，也表示
// terminal 累计快照存在，调用方不得回退到较早的 assistant usage。
func GoalUsageFromTokenUsageWithPresence(
	usage sdkprotocol.TokenUsage,
) (protocol.GoalUsage, bool) {
	goalUsage := protocol.GoalUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		ReasoningTokens:          usage.ReasoningTokens,
	}
	supplementGoalUsageFromNestedRaw(&goalUsage, usage.Raw)
	if providerTotal, ok := tokenUsageProviderTotal(usage); ok {
		goalUsage.ActualTotalTokens = providerTotal
		goalUsage.ActualTotalKnown = true
	} else if usage.TotalTokens > 0 {
		// bridge 为兼容缺少 total_tokens 的 provider 会合成 TokenUsage.TotalTokens；
		// 该值没有 provider provenance，必须按 breakdown 重算并明确标记为估算。
		goalUsage.ActualTokensEstimated = true
	}
	return goalUsage.NormalizeTotals(), tokenUsageHasObservedData(usage)
}

// GoalUsageFromRaw 从动态 usage JSON 提取 Goal accounting usage。
func GoalUsageFromRaw(raw any) (protocol.GoalUsage, bool) {
	usage, ok := sdkprotocol.ParseTokenUsage(raw)
	if !ok {
		return protocol.GoalUsage{}, false
	}
	goalUsage, _ := GoalUsageFromTokenUsageWithPresence(usage)
	return goalUsage, true
}

func tokenUsageHasObservedData(usage sdkprotocol.TokenUsage) bool {
	if _, ok := tokenUsageProviderTotal(usage); ok {
		return true
	}
	if !usage.IsZero() || rawTokenUsageHasObservedData(usage.Raw) {
		return true
	}
	return false
}

func tokenUsageProviderTotal(usage sdkprotocol.TokenUsage) (int64, bool) {
	// 手工构造的 typed usage 没有 Raw；保持该公共转换入口的兼容语义，
	// 生产 parser 则总会携带 Raw，可据此区分 provider total 与 bridge fallback。
	if len(usage.Raw) == 0 {
		return max(usage.TotalTokens, 0), usage.TotalTokens > 0
	}
	return rawTokenUsageExactTotal(usage.Raw)
}

func rawTokenUsageProviderTotal(raw map[string]any) (int64, bool) {
	for _, key := range []string{"total_tokens", "totalTokenCount"} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		parsed, parsedOK := sdkprotocol.ParseTokenUsage(map[string]any{"total_tokens": value})
		if !parsedOK {
			return 0, false
		}
		total := max(parsed.TotalTokens, 0)
		// 有正数 breakdown 时，provider total=0 是自相矛盾的占位/缺省值，
		// 不能把已经观察到的 Goal 用量覆盖成权威零。
		if total == 0 && rawTokenUsageHasPositiveBreakdown(raw) {
			return 0, false
		}
		return total, true
	}
	return 0, false
}

// rawTokenUsageExactTotal 只接受能追溯到 provider 的 total。nxs 的
// ModelUsage 会在外层写入 input+output 合成 total，并把 provider 原始 usage
// 放进 raw；原始 usage 没有 total 时，外层合成值只能作为 breakdown 估算。
func rawTokenUsageExactTotal(raw map[string]any) (int64, bool) {
	if providerRaw, ok := nestedUsageMap(raw["raw"]); ok && rawTokenUsageHasObservedData(providerRaw) {
		if total, hasProviderTotal := rawTokenUsageProviderTotal(providerRaw); hasProviderTotal {
			return total, true
		}
		if total, hasOuterTotal := rawTokenUsageProviderTotal(raw); hasOuterTotal &&
			!rawTokenUsageTotalMatchesInputOutput(raw, total) {
			return total, true
		}
		return 0, false
	}

	if total, ok := rawTokenUsageProviderTotal(raw); ok {
		return total, true
	}

	// result.model_usage 会把多个 provider/model usage 放在外层 map；
	// 只有每个参与聚合的子 usage 都携带 provider total 时，总和才是精确值。
	var total int64
	foundNestedUsage := false
	for key, value := range raw {
		if key == "raw" {
			continue
		}
		nested, ok := nestedUsageMap(value)
		if !ok || !rawTokenUsageHasObservedData(nested) {
			continue
		}
		foundNestedUsage = true
		nestedTotal, exact := rawTokenUsageExactTotal(nested)
		if !exact {
			return 0, false
		}
		total += nestedTotal
	}
	return total, foundNestedUsage
}

func rawTokenUsageTotalMatchesInputOutput(raw map[string]any, total int64) bool {
	parsed, ok := sdkprotocol.ParseTokenUsage(raw)
	if !ok {
		return false
	}
	return max(total, 0) == max(parsed.InputTokens, 0)+max(parsed.OutputTokens, 0)
}

func rawTokenUsageHasObservedData(raw map[string]any) bool {
	if len(raw) == 0 {
		return false
	}
	if _, ok := rawTokenUsageProviderTotal(raw); ok || rawTokenUsageHasBreakdown(raw) {
		return true
	}
	if providerRaw, ok := nestedUsageMap(raw["raw"]); ok && rawTokenUsageHasObservedData(providerRaw) {
		return true
	}
	for key, value := range raw {
		if key == "raw" {
			continue
		}
		nested, ok := nestedUsageMap(value)
		if ok && rawTokenUsageHasObservedData(nested) {
			return true
		}
	}
	return false
}

func rawTokenUsageHasBreakdown(raw map[string]any) bool {
	for _, key := range []string{
		"input_tokens",
		"prompt_tokens",
		"output_tokens",
		"completion_tokens",
		"cache_creation_input_tokens",
		"cache_creation_tokens",
		"cache_read_input_tokens",
		"cache_read_tokens",
		"reasoning_tokens",
		"reasoning_output_tokens",
		"promptTokenCount",
		"candidatesTokenCount",
		"cachedContentTokenCount",
		"thoughtsTokenCount",
	} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func rawTokenUsageHasPositiveBreakdown(raw map[string]any) bool {
	usage := providerRawGoalUsageBreakdown(raw)
	return usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.ReasoningTokens > 0
}

func supplementGoalUsageFromNestedRaw(goalUsage *protocol.GoalUsage, raw map[string]any) {
	if goalUsage == nil || len(raw) == 0 {
		return
	}
	supplement, ok := nestedRawGoalUsageBreakdown(raw)
	if !ok {
		return
	}
	goalUsage.InputTokens = max(goalUsage.InputTokens, supplement.InputTokens)
	goalUsage.OutputTokens = max(goalUsage.OutputTokens, supplement.OutputTokens)
	goalUsage.CacheCreationInputTokens = max(
		goalUsage.CacheCreationInputTokens,
		supplement.CacheCreationInputTokens,
	)
	goalUsage.CacheReadInputTokens = max(
		goalUsage.CacheReadInputTokens,
		supplement.CacheReadInputTokens,
	)
	goalUsage.ReasoningTokens = max(goalUsage.ReasoningTokens, supplement.ReasoningTokens)
}

func nestedRawGoalUsageBreakdown(raw map[string]any) (protocol.GoalUsage, bool) {
	if providerRaw, ok := nestedUsageMap(raw["raw"]); ok {
		return providerRawGoalUsageBreakdown(providerRaw), rawTokenUsageHasObservedData(providerRaw)
	}

	var (
		aggregate protocol.GoalUsage
		found     bool
	)
	for _, value := range raw {
		modelUsage, ok := nestedUsageMap(value)
		if !ok {
			continue
		}
		providerRaw, ok := nestedUsageMap(modelUsage["raw"])
		if !ok || !rawTokenUsageHasObservedData(providerRaw) {
			continue
		}
		aggregate = addGoalUsageBreakdown(aggregate, providerRawGoalUsageBreakdown(providerRaw))
		found = true
	}
	return aggregate, found
}

func providerRawGoalUsageBreakdown(raw map[string]any) protocol.GoalUsage {
	parsed, _ := sdkprotocol.ParseTokenUsage(raw)
	usage := protocol.GoalUsage{
		InputTokens:              parsed.InputTokens,
		OutputTokens:             parsed.OutputTokens,
		CacheCreationInputTokens: parsed.CacheCreationInputTokens,
		CacheReadInputTokens:     parsed.CacheReadInputTokens,
		ReasoningTokens:          parsed.ReasoningTokens,
	}
	usage.InputTokens = max(usage.InputTokens, protocol.Int64FromAny(raw["promptTokenCount"]))
	usage.OutputTokens = max(usage.OutputTokens, protocol.Int64FromAny(raw["candidatesTokenCount"]))
	usage.CacheReadInputTokens = max(
		usage.CacheReadInputTokens,
		protocol.Int64FromAny(raw["cachedContentTokenCount"]),
	)
	usage.ReasoningTokens = max(
		usage.ReasoningTokens,
		protocol.Int64FromAny(raw["thoughtsTokenCount"]),
	)
	return usage
}

func addGoalUsageBreakdown(left protocol.GoalUsage, right protocol.GoalUsage) protocol.GoalUsage {
	left.InputTokens += max(right.InputTokens, 0)
	left.OutputTokens += max(right.OutputTokens, 0)
	left.CacheCreationInputTokens += max(right.CacheCreationInputTokens, 0)
	left.CacheReadInputTokens += max(right.CacheReadInputTokens, 0)
	left.ReasoningTokens += max(right.ReasoningTokens, 0)
	return left
}

func nestedUsageMap(value any) (map[string]any, bool) {
	nested, ok := value.(map[string]any)
	return nested, ok && len(nested) > 0
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
