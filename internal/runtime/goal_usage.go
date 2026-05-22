package runtime

import (
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
