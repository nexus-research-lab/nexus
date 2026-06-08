package runtime

import (
	"fmt"
	"strings"
	"unicode/utf8"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
	case sdkprotocol.MessageTypeTaskProgress:
		fields = append(fields, buildTaskProgressFields(message)...)
	case sdkprotocol.MessageTypeSystem:
		fields = append(fields, buildSystemMessageFields(message)...)
	}
	return fields
}

// BuildSDKMessageLogSummary 生成适合调试视图的单行摘要。
func BuildSDKMessageLogSummary(message sdkprotocol.ReceivedMessage) string {
	switch message.Type {
	case sdkprotocol.MessageTypeStreamEvent:
		return summarizeStreamMessage(message)
	case sdkprotocol.MessageTypeUser:
		return summarizeUserMessage(message)
	case sdkprotocol.MessageTypeAssistant:
		return summarizeAssistantMessage(message)
	case sdkprotocol.MessageTypeResult:
		return summarizeResultMessage(message)
	case sdkprotocol.MessageTypeSystem:
		return summarizeSystemMessage(message)
	case sdkprotocol.MessageTypeTaskProgress:
		return summarizeTaskProgressMessage(message)
	default:
		return string(message.Type)
	}
}

func buildUserMessageFields(message sdkprotocol.ReceivedMessage, includeSnapshotData bool) []any {
	if message.User == nil {
		return nil
	}
	toolResults := 0
	toolErrors := 0
	for _, block := range message.User.Message.Content {
		toolResultBlock, ok := sdkprotocol.AsToolResultBlock(block)
		if !ok {
			continue
		}
		toolResults++
		if toolResultBlock.IsError {
			toolErrors++
		}
	}
	fields := []any{}
	if toolResults > 0 {
		fields = append(fields, "tool_results", toolResults)
	}
	if toolErrors > 0 {
		fields = append(fields, "tool_errors", toolErrors)
	}
	if includeSnapshotData {
		fields = append(fields, buildContentSnapshotFields("user", message.User.Message.Content)...)
	}
	return fields
}

func buildAssistantMessageFields(message sdkprotocol.ReceivedMessage, includeSnapshotData bool) []any {
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
	if includeSnapshotData {
		fields = append(fields, buildContentSnapshotFields("assistant", message.Assistant.Message.Content)...)
	}
	return fields
}

func buildContentSnapshotFields(prefix string, blocks []sdkprotocol.ContentBlock) []any {
	textParts := make([]string, 0, len(blocks))
	thinkingParts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if textBlock, ok := sdkprotocol.AsTextBlock(block); ok {
			if text := messageDebugText(textBlock.Text); text != "" {
				textParts = append(textParts, text)
			}
			continue
		}
		if thinkingBlock, ok := sdkprotocol.AsThinkingBlock(block); ok {
			if thinking := messageDebugText(thinkingBlock.Thinking); thinking != "" {
				thinkingParts = append(thinkingParts, thinking)
			}
		}
	}
	fields := []any{}
	if len(textParts) > 0 {
		fields = append(fields, prefix+"_text", strings.Join(textParts, "\n\n"))
	}
	if len(thinkingParts) > 0 {
		fields = append(fields, prefix+"_thinking", strings.Join(thinkingParts, "\n\n"))
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
	if message.Stream == nil {
		return nil
	}
	event := rawMap(message.Stream.Event)
	if len(event) == 0 {
		event = rawMap(message.Stream.Data)
	}
	eventType := strings.TrimSpace(rawString(event["type"]))
	if eventType == "" {
		return nil
	}
	fields := []any{"stream_event", eventType}
	fields = appendRawLogField(fields, "stream_index", event["index"])
	switch eventType {
	case "message_start":
		startMessage := rawMap(event["message"])
		fields = appendRawLogField(fields, "stream_role", startMessage["role"])
		fields = appendRawLogField(fields, "stream_model", startMessage["model"])
	case "content_block_start":
		block := rawMap(event["content_block"])
		blockType := normalizeSDKBlockType(rawString(block["type"]))
		fields = appendRawLogField(fields, "stream_block", blockType)
		switch blockType {
		case "text":
			if text := streamDebugText(rawString(block["text"])); text != "" {
				fields = append(fields, "stream_text", text)
			}
		case "tool_use":
			if toolName := safeToolName(firstNonEmpty(rawString(block["name"]), rawString(block["id"]))); toolName != "" {
				fields = append(fields, "tool", toolName)
			}
		}
	case "content_block_delta":
		delta := rawMap(event["delta"])
		deltaType := strings.TrimSpace(rawString(delta["type"]))
		fields = appendRawLogField(fields, "stream_delta", deltaType)
		switch deltaType {
		case "text_delta":
			text := rawString(delta["text"])
			if preview := streamDebugText(text); preview != "" {
				fields = append(fields, "delta", preview)
			}
		case "thinking_delta":
			text := firstNonEmpty(rawString(delta["thinking"]), rawString(delta["text"]))
			if preview := streamDebugText(text); preview != "" {
				fields = append(fields, "thinking", preview)
			}
		}
	case "content_block_stop":
	case "message_delta":
		delta := rawMap(event["delta"])
		fields = appendRawLogField(fields, "stream_stop_reason", delta["stop_reason"])
		fields = appendRawLogField(fields, "stream_stop_sequence", delta["stop_sequence"])
	case "message_stop":
	}
	return fields
}

func buildTaskProgressFields(message sdkprotocol.ReceivedMessage) []any {
	if message.TaskProgress == nil {
		return nil
	}
	fields := []any{}
	if toolName := safeToolName(message.TaskProgress.LastToolName); toolName != "" {
		fields = append(fields, "tool", toolName)
	}
	if message.TaskProgress.Usage.DurationMS > 0 {
		fields = append(fields, "duration_ms", message.TaskProgress.Usage.DurationMS)
	}
	return fields
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
				"cmd", strings.TrimSpace(message.System.Init.CWD),
				"permission_mode", strings.TrimSpace(string(message.System.Init.PermissionMode)),
				"skills", strings.Join(message.System.Init.Skills, ","),
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
	if len(event) == 0 {
		event = rawMap(message.Stream.Data)
	}
	eventType := strings.TrimSpace(rawString(event["type"]))
	if eventType == "" {
		return "stream"
	}
	preview := ""
	switch eventType {
	case "message_start":
		startMessage := rawMap(event["message"])
		role := strings.TrimSpace(rawString(startMessage["role"]))
		if role != "" {
			return "stream message_start(" + role + ")"
		}
	case "content_block_delta":
		delta := rawMap(event["delta"])
		deltaType := strings.TrimSpace(rawString(delta["type"]))
		if deltaType != "" {
			return "stream content_block_delta(" + deltaType + ")"
		}
	case "content_block_start":
		block := rawMap(event["content_block"])
		blockType := normalizeSDKBlockType(rawString(block["type"]))
		if blockType != "" {
			if blockType == "tool_use" {
				preview = safeToolName(rawString(block["name"]))
			}
			return appendSummaryPreview("stream content_block_start("+blockType+")", preview)
		}
	case "message_delta":
		delta := rawMap(event["delta"])
		stopReason := strings.TrimSpace(rawString(delta["stop_reason"]))
		if stopReason != "" {
			return "stream message_delta(stop_reason=" + stopReason + ")"
		}
	}
	return appendSummaryPreview("stream "+eventType, preview)
}

func summarizeUserMessage(message sdkprotocol.ReceivedMessage) string {
	if message.User == nil {
		return "user"
	}
	blockTypes, preview := summarizeContentBlocks(message.User.Message.Content)
	if len(blockTypes) == 0 {
		return "user"
	}
	return appendSummaryPreview("user snapshot("+strings.Join(blockTypes, ",")+")", preview)
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

func summarizeTaskProgressMessage(message sdkprotocol.ReceivedMessage) string {
	if message.TaskProgress == nil {
		return "task_progress"
	}
	toolName := strings.TrimSpace(message.TaskProgress.LastToolName)
	if toolName == "" {
		return "task_progress"
	}
	return "task_progress " + toolName
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

func appendRawLogField(fields []any, key string, value any) []any {
	if strings.TrimSpace(key) == "" {
		return fields
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fields
		}
		return append(fields, key, strings.TrimSpace(typed))
	case int:
		return append(fields, key, typed)
	case int8:
		return append(fields, key, typed)
	case int16:
		return append(fields, key, typed)
	case int32:
		return append(fields, key, typed)
	case int64:
		return append(fields, key, typed)
	case uint:
		return append(fields, key, typed)
	case uint8:
		return append(fields, key, typed)
	case uint16:
		return append(fields, key, typed)
	case uint32:
		return append(fields, key, typed)
	case uint64:
		return append(fields, key, typed)
	case float32:
		return append(fields, key, typed)
	case float64:
		return append(fields, key, typed)
	case bool:
		return append(fields, key, typed)
	default:
		return fields
	}
}

func streamDebugText(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "" {
		return ""
	}
	const maxRunes = 240
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "..."
}

func messageDebugText(value string) string {
	return strings.TrimSpace(value)
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
