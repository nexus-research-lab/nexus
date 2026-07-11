package trace

import (
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type streamEventFieldBuilder func([]any, map[string]any) []any

var streamEventFieldBuilders = map[string]streamEventFieldBuilder{
	"message_start":       appendMessageStartFields,
	"content_block_start": appendContentBlockStartFields,
	"content_block_delta": appendContentBlockDeltaFields,
	"message_delta":       appendMessageDeltaFields,
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
			if text := strings.TrimSpace(textBlock.Text); text != "" {
				textParts = append(textParts, text)
			}
			continue
		}
		if thinkingBlock, ok := sdkprotocol.AsThinkingBlock(block); ok {
			if thinking := strings.TrimSpace(thinkingBlock.Thinking); thinking != "" {
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
	event := RawMap(message.Stream.Event)
	if len(event) == 0 {
		event = RawMap(message.Stream.Data)
	}
	eventType := strings.TrimSpace(RawString(event["type"]))
	if eventType == "" {
		return nil
	}
	fields := []any{"stream_event", eventType}
	fields = appendRawLogField(fields, "stream_index", event["index"])
	if builder := streamEventFieldBuilders[eventType]; builder != nil {
		fields = builder(fields, event)
	}
	return fields
}

func appendMessageStartFields(fields []any, event map[string]any) []any {
	message := RawMap(event["message"])
	fields = appendRawLogField(fields, "stream_role", message["role"])
	return appendRawLogField(fields, "stream_model", message["model"])
}

func appendContentBlockStartFields(fields []any, event map[string]any) []any {
	block := RawMap(event["content_block"])
	blockType := normalizeSDKBlockType(RawString(block["type"]))
	fields = appendRawLogField(fields, "stream_block", blockType)
	switch blockType {
	case "text":
		if text := streamDebugText(RawString(block["text"])); text != "" {
			fields = append(fields, "stream_text", text)
		}
	case "tool_use":
		toolName := strings.TrimSpace(FirstNonEmpty(RawString(block["name"]), RawString(block["id"])))
		if toolName != "" {
			fields = append(fields, "tool", toolName)
		}
	}
	return fields
}

func appendContentBlockDeltaFields(fields []any, event map[string]any) []any {
	delta := RawMap(event["delta"])
	deltaType := strings.TrimSpace(RawString(delta["type"]))
	fields = appendRawLogField(fields, "stream_delta", deltaType)
	var key, text string
	switch deltaType {
	case "text_delta":
		key, text = "delta", RawString(delta["text"])
	case "thinking_delta":
		key = "thinking"
		text = FirstNonEmpty(RawString(delta["thinking"]), RawString(delta["text"]))
	}
	if preview := streamDebugText(text); key != "" && preview != "" {
		fields = append(fields, key, preview)
	}
	return fields
}

func appendMessageDeltaFields(fields []any, event map[string]any) []any {
	delta := RawMap(event["delta"])
	fields = appendRawLogField(fields, "stream_stop_reason", delta["stop_reason"])
	return appendRawLogField(fields, "stream_stop_sequence", delta["stop_sequence"])
}

func buildTaskProgressFields(message sdkprotocol.ReceivedMessage) []any {
	if message.TaskProgress == nil {
		return nil
	}
	fields := []any{}
	if toolName := strings.TrimSpace(message.TaskProgress.LastToolName); toolName != "" {
		fields = append(fields, "tool", toolName)
	}
	if message.TaskProgress.Usage.DurationMS > 0 {
		fields = append(fields, "duration_ms", message.TaskProgress.Usage.DurationMS)
	}
	return fields
}

func buildToolProgressFields(message sdkprotocol.ReceivedMessage) []any {
	if message.ToolProgress == nil {
		return nil
	}
	fields := []any{}
	if toolName := strings.TrimSpace(message.ToolProgress.ToolName); toolName != "" {
		fields = append(fields, "tool", toolName)
	}
	if taskID := strings.TrimSpace(message.ToolProgress.TaskID); taskID != "" {
		fields = append(fields, "task_id", taskID)
	}
	if toolUseID := strings.TrimSpace(message.ToolProgress.ToolUseID); toolUseID != "" {
		fields = append(fields, "tool_use_id", toolUseID)
	}
	if message.ToolProgress.ParentToolUseID != nil {
		if parentToolUseID := strings.TrimSpace(*message.ToolProgress.ParentToolUseID); parentToolUseID != "" {
			fields = append(fields, "parent_tool_use_id", parentToolUseID)
		}
	}
	return fields
}

func buildToolUseSummaryFields(message sdkprotocol.ReceivedMessage) []any {
	if message.ToolUseSummary == nil {
		return nil
	}
	fields := []any{}
	if summary := strings.TrimSpace(message.ToolUseSummary.Summary); summary != "" {
		fields = append(fields, "tool_summary", streamDebugText(summary))
	}
	if count := len(message.ToolUseSummary.PrecedingToolUseIDs); count > 0 {
		fields = append(fields, "tool_summary_count", count)
	}
	return fields
}

func buildRateLimitEventFields(message sdkprotocol.ReceivedMessage) []any {
	if message.RateLimit == nil || len(message.RateLimit.RateLimitInfo) == 0 {
		return nil
	}
	return []any{"rate_limit_info", message.RateLimit.RateLimitInfo}
}

func buildPromptSuggestionFields(message sdkprotocol.ReceivedMessage) []any {
	if message.PromptSuggestion == nil {
		return nil
	}
	if suggestion := strings.TrimSpace(message.PromptSuggestion.Suggestion); suggestion != "" {
		return []any{"prompt_suggestion", streamDebugText(suggestion)}
	}
	return nil
}

func buildAuthStatusFields(message sdkprotocol.ReceivedMessage) []any {
	if message.AuthStatus == nil {
		return nil
	}
	fields := []any{"auth_is_authenticating", message.AuthStatus.IsAuthenticating}
	if outputCount := len(message.AuthStatus.Output); outputCount > 0 {
		fields = append(fields, "auth_output_count", outputCount)
	}
	if errorText := strings.TrimSpace(message.AuthStatus.Error); errorText != "" {
		fields = append(fields, "auth_error", streamDebugText(errorText))
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
