package runtime

import sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

// SDKMessageLogOptions 控制 SDK 消息调试日志的输出范围。
type SDKMessageLogOptions struct {
	IncludeStreamEvent  bool
	IncludeSnapshotData bool
}

// DefaultSDKMessageLogOptions 返回兼容历史行为的 SDK 消息日志选项。
func DefaultSDKMessageLogOptions() SDKMessageLogOptions {
	return SDKMessageLogOptions{
		IncludeStreamEvent:  true,
		IncludeSnapshotData: true,
	}
}

// BuildSDKMessageLogFields 生成 SDK 消息调试日志字段。
func BuildSDKMessageLogFields(message sdkprotocol.ReceivedMessage) []any {
	return BuildSDKMessageLogFieldsWithOptions(message, DefaultSDKMessageLogOptions())
}

// BuildSDKMessageLogFieldsWithOptions 按选项生成 SDK 消息调试日志字段。
func BuildSDKMessageLogFieldsWithOptions(
	message sdkprotocol.ReceivedMessage,
	options SDKMessageLogOptions,
) []any {
	fields := []any{
		"sdk_summary", BuildSDKMessageLogSummary(message),
	}

	switch message.Type {
	case sdkprotocol.MessageTypeUser:
		fields = append(fields, buildUserMessageFields(message, options.IncludeSnapshotData)...)
	case sdkprotocol.MessageTypeAssistant:
		fields = append(fields, buildAssistantMessageFields(message, options.IncludeSnapshotData)...)
	case sdkprotocol.MessageTypeResult:
		fields = append(fields, buildResultMessageFields(message)...)
	case sdkprotocol.MessageTypeStreamEvent:
		if !options.IncludeStreamEvent {
			return nil
		}
		fields = append(fields, buildStreamEventFields(message)...)
	case sdkprotocol.MessageTypeToolProgress:
		fields = append(fields, buildToolProgressFields(message)...)
	case sdkprotocol.MessageTypeToolUseSummary:
		fields = append(fields, buildToolUseSummaryFields(message)...)
	case sdkprotocol.MessageTypeTaskProgress:
		fields = append(fields, buildTaskProgressFields(message)...)
	case sdkprotocol.MessageTypeRateLimitEvent:
		fields = append(fields, buildRateLimitEventFields(message)...)
	case sdkprotocol.MessageTypePromptSuggestion:
		fields = append(fields, buildPromptSuggestionFields(message)...)
	case sdkprotocol.MessageTypeAuthStatus:
		fields = append(fields, buildAuthStatusFields(message)...)
	case sdkprotocol.MessageTypeSystem:
		fields = append(fields, buildSystemMessageFields(message)...)
	}
	return fields
}
