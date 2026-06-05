package provider

import "strings"

const (
	// RuntimeKindClaude 表示 Claude Code runtime。
	RuntimeKindClaude = "claude"
	// RuntimeKindNXS 表示 Nexus 原生 runtime。
	RuntimeKindNXS = "nxs"
)

const (
	// APIFormatAnthropicMessages 表示 Anthropic Messages 协议。
	APIFormatAnthropicMessages = "anthropic_messages"
	// APIFormatChatCompletions 表示 OpenAI Chat Completions 协议。
	APIFormatChatCompletions = "chat_completions"
)

// NormalizeRuntimeKind 归一化用户可配置的 runtime kind。
func NormalizeRuntimeKind(runtimeKind string) string {
	switch strings.ToLower(strings.TrimSpace(runtimeKind)) {
	case RuntimeKindNXS, "go", "go-native", "gonative":
		return RuntimeKindNXS
	case RuntimeKindClaude, "claude-code", "claudecode":
		return RuntimeKindClaude
	}
	return RuntimeKindClaude
}

// SupportsAPIFormat 判断指定 runtime 是否原生支持某个 provider API format。
func SupportsAPIFormat(runtimeKind string, apiFormat string) bool {
	switch strings.TrimSpace(apiFormat) {
	case "", APIFormatAnthropicMessages:
		return true
	case APIFormatChatCompletions:
		return NormalizeRuntimeKind(runtimeKind) == RuntimeKindNXS
	default:
		return false
	}
}

// SupportsAnyRuntime 判断至少一个 Agent runtime 是否支持该 provider API format。
func SupportsAnyRuntime(apiFormat string) bool {
	return SupportsAPIFormat(RuntimeKindClaude, apiFormat) ||
		SupportsAPIFormat(RuntimeKindNXS, apiFormat)
}
