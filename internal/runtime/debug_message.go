package runtime

import (
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// BuildSDKMessageLogFields 生成 SDK 消息调试日志字段。
func BuildSDKMessageLogFields(message sdkprotocol.ReceivedMessage) []any {
	fields := []any{
		"sdk_summary", BuildSDKMessageLogSummary(message),
	}

	switch message.Type {
	case sdkprotocol.MessageTypeAssistant:
		fields = append(fields, buildAssistantMessageFields(message)...)
	case sdkprotocol.MessageTypeResult:
		fields = append(fields, buildResultMessageFields(message)...)
	case sdkprotocol.MessageTypeStreamEvent:
		fields = append(fields, buildStreamEventFields(message)...)
	case sdkprotocol.MessageTypeToolProgress:
		fields = append(fields, buildToolProgressFields(message)...)
	case sdkprotocol.MessageTypeSystem:
		fields = append(fields, buildSystemMessageFields(message)...)
	case sdkprotocol.MessageTypeAuthStatus:
		if message.AuthStatus != nil {
			fields = append(
				fields,
				"is_authenticating", message.AuthStatus.IsAuthenticating,
				"auth_error", strings.TrimSpace(message.AuthStatus.Error),
			)
		}
	}
	return fields
}

// BuildSDKMessageLogSummary 生成适合调试视图的单行摘要。
func BuildSDKMessageLogSummary(message sdkprotocol.ReceivedMessage) string {
	switch message.Type {
	case sdkprotocol.MessageTypeStreamEvent:
		return summarizeStreamMessage(message)
	case sdkprotocol.MessageTypeAssistant:
		return summarizeAssistantMessage(message)
	case sdkprotocol.MessageTypeResult:
		return summarizeResultMessage(message)
	case sdkprotocol.MessageTypeSystem:
		return summarizeSystemMessage(message)
	case sdkprotocol.MessageTypeToolProgress:
		return summarizeToolProgressMessage(message)
	case sdkprotocol.MessageTypePromptSuggestion:
		return "prompt_suggestion"
	case sdkprotocol.MessageTypeAuthStatus:
		return "auth_status"
	default:
		return string(message.Type)
	}
}

func buildAssistantMessageFields(message sdkprotocol.ReceivedMessage) []any {
	if message.Assistant == nil {
		return nil
	}
	fields := []any{}
	if model := strings.TrimSpace(message.Assistant.Message.Model); model != "" {
		fields = append(fields, "assistant_model", model)
	}
	if stopReason := strings.TrimSpace(fmt.Sprint(message.Assistant.Message.StopReason)); stopReason != "" && stopReason != "<nil>" {
		fields = append(fields, "assistant_stop_reason", stopReason)
	}
	if errText := strings.TrimSpace(message.Assistant.Error); errText != "" {
		fields = append(fields, "assistant_error", errText)
	}
	return fields
}

func buildResultMessageFields(message sdkprotocol.ReceivedMessage) []any {
	if message.Result == nil {
		return nil
	}
	fields := []any{
		"result_is_error", message.Result.IsError,
		"result_num_turns", message.Result.NumTurns,
	}
	if terminalReason := strings.TrimSpace(message.Result.TerminalReason); terminalReason != "" {
		fields = append(fields, "result_terminal_reason", terminalReason)
	}
	if stopReason := strings.TrimSpace(fmt.Sprint(message.Result.StopReason)); stopReason != "" && stopReason != "<nil>" {
		fields = append(fields, "result_stop_reason", stopReason)
	}
	if len(message.Result.Errors) > 0 {
		fields = append(fields, "result_error_count", len(message.Result.Errors))
	}
	return fields
}

func buildStreamEventFields(message sdkprotocol.ReceivedMessage) []any {
	return nil
}

func buildToolProgressFields(message sdkprotocol.ReceivedMessage) []any {
	return nil
}

func buildSystemMessageFields(message sdkprotocol.ReceivedMessage) []any {
	if message.System == nil {
		return nil
	}
	fields := []any{
		"system_subtype", strings.TrimSpace(message.System.Subtype),
	}
	switch message.System.Subtype {
	case "init":
		if message.System.Init != nil {
			fields = append(
				fields,
				"system_model", strings.TrimSpace(message.System.Init.Model),
				"system_permission_mode", strings.TrimSpace(string(message.System.Init.PermissionMode)),
			)
		}
	case "status":
		if message.System.Status != nil {
			fields = append(
				fields,
				"system_status", strings.TrimSpace(message.System.Status.Status),
				"system_permission_mode", strings.TrimSpace(string(message.System.Status.PermissionMode)),
			)
		}
	case "task_started":
	case "task_progress":
	case "task_notification":
	}
	return fields
}

func summarizeStreamMessage(message sdkprotocol.ReceivedMessage) string {
	if message.Stream == nil {
		return "stream"
	}
	event := rawMap(message.Stream.Event)
	eventType := strings.TrimSpace(rawString(event["type"]))
	if eventType == "" {
		return "stream"
	}
	preview := ""
	switch eventType {
	case "content_block_delta":
		delta := rawMap(event["delta"])
		deltaType := strings.TrimSpace(rawString(delta["type"]))
		if deltaType != "" {
			return fmt.Sprintf("stream %s(%s)", eventType, deltaType)
		}
	case "content_block_start":
		block := rawMap(event["content_block"])
		blockType := strings.TrimSpace(rawString(block["type"]))
		if blockType != "" {
			if blockType == "tool_use" {
				preview = safeToolName(rawString(block["name"]))
			}
			return appendSummaryPreview(fmt.Sprintf("stream %s(%s)", eventType, blockType), preview)
		}
	}
	return appendSummaryPreview("stream "+eventType, preview)
}

func summarizeAssistantMessage(message sdkprotocol.ReceivedMessage) string {
	if message.Assistant == nil {
		return "assistant"
	}
	blockTypes, preview := summarizeContentBlocks(message.Assistant.Message.Content)
	if len(blockTypes) == 0 {
		return "assistant snapshot"
	}
	return appendSummaryPreview("assistant snapshot("+strings.Join(blockTypes, ",")+")", preview)
}

func summarizeResultMessage(message sdkprotocol.ReceivedMessage) string {
	if message.Result == nil {
		return "result"
	}
	subtype := strings.TrimSpace(message.Result.Subtype)
	if subtype == "" {
		return "result"
	}
	return "result " + subtype
}

func summarizeSystemMessage(message sdkprotocol.ReceivedMessage) string {
	if message.System == nil {
		return "system"
	}
	subtype := strings.TrimSpace(message.System.Subtype)
	if subtype == "" {
		return "system"
	}
	switch subtype {
	case "task_progress":
		if message.System.TaskProgress != nil {
			return "system task_progress"
		}
	case "task_started":
		if message.System.TaskStarted != nil {
			return "system task_started"
		}
	case "task_notification":
		if message.System.TaskNotification != nil {
			return "system task_notification"
		}
	}
	return "system " + subtype
}

func summarizeToolProgressMessage(message sdkprotocol.ReceivedMessage) string {
	if message.ToolProgress == nil {
		return "tool_progress"
	}
	toolName := strings.TrimSpace(message.ToolProgress.ToolName)
	if toolName == "" {
		return "tool_progress"
	}
	return "tool_progress " + toolName
}

func appendSummaryPreview(summary string, preview string) string {
	summary = strings.TrimSpace(summary)
	preview = strings.TrimSpace(preview)
	if summary == "" || preview == "" {
		return summary
	}
	return summary + " \"" + preview + "\""
}

func summarizeContentBlocks(blocks []sdkprotocol.ContentBlock) ([]string, string) {
	blockTypes := make([]string, 0, len(blocks))
	previewParts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		blockType := normalizeSDKBlockType(string(block.Type()))
		if blockType == "" {
			blockType = "unknown"
		}
		blockTypes = append(blockTypes, blockType)
		switch blockType {
		case "tool_use":
			if toolUseBlock, ok := sdkprotocol.AsToolUseBlock(block); ok {
				if toolName := safeToolName(firstNonEmpty(toolUseBlock.Name, toolUseBlock.ID)); toolName != "" {
					previewParts = append(previewParts, "tool_use:"+toolName)
				}
			}
		}
	}
	return blockTypes, strings.Join(previewParts, " | ")
}

func normalizeSDKBlockType(blockType string) string {
	switch strings.TrimSpace(blockType) {
	case "server_tool_use":
		return "tool_use"
	case "server_tool_result":
		return "tool_result"
	default:
		return strings.TrimSpace(blockType)
	}
}

func rawMap(value any) map[string]any {
	if payload, ok := value.(map[string]any); ok {
		return payload
	}
	return map[string]any{}
}

func rawString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func safeToolName(value string) string {
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
