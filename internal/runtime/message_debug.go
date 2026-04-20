// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：message_debug.go
// @Date   ：2026/04/16 15:53:00
// @Author ：leemysw
// 2026/04/16 15:53:00   Create
// =====================================================

package runtime

import (
	"encoding/json"
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

const sdkMessagePreviewLimit = 240

// BuildSDKMessageLogFields 生成 SDK 消息调试日志字段。
func BuildSDKMessageLogFields(message sdkprotocol.ReceivedMessage) []any {
	fields := []any{
		"sdk_summary", BuildSDKMessageLogSummary(message),
		"sdk_message_type", string(message.Type),
	}
	if subtype := strings.TrimSpace(message.Subtype); subtype != "" {
		fields = append(fields, "sdk_message_subtype", subtype)
	}
	if sessionID := strings.TrimSpace(message.SessionID); sessionID != "" {
		fields = append(fields, "sdk_session_id", sessionID)
	}
	if uuid := strings.TrimSpace(message.UUID); uuid != "" {
		fields = append(fields, "sdk_message_uuid", uuid)
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
	case sdkprotocol.MessageTypePromptSuggestion:
		if message.PromptSuggestion != nil {
			fields = append(fields, "suggestion_preview", truncateForLog(message.PromptSuggestion.Suggestion))
		}
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
	fields := []any{
		"assistant_message_id", strings.TrimSpace(message.Assistant.Message.ID),
	}
	if model := strings.TrimSpace(message.Assistant.Message.Model); model != "" {
		fields = append(fields, "assistant_model", model)
	}
	if stopReason := strings.TrimSpace(fmt.Sprint(message.Assistant.Message.StopReason)); stopReason != "" && stopReason != "<nil>" {
		fields = append(fields, "assistant_stop_reason", stopReason)
	}
	blockTypes, preview := summarizeContentBlocks(message.Assistant.Message.Content)
	if len(blockTypes) > 0 {
		fields = append(fields, "assistant_block_types", strings.Join(blockTypes, ","))
	}
	if preview != "" {
		fields = append(fields, "assistant_preview", preview)
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
		"result_subtype", strings.TrimSpace(message.Result.Subtype),
		"result_is_error", message.Result.IsError,
		"result_num_turns", message.Result.NumTurns,
	}
	if terminalReason := strings.TrimSpace(message.Result.TerminalReason); terminalReason != "" {
		fields = append(fields, "result_terminal_reason", terminalReason)
	}
	if stopReason := strings.TrimSpace(fmt.Sprint(message.Result.StopReason)); stopReason != "" && stopReason != "<nil>" {
		fields = append(fields, "result_stop_reason", stopReason)
	}
	if resultPreview := truncateForLog(message.Result.Result); resultPreview != "" {
		fields = append(fields, "result_preview", resultPreview)
	}
	if len(message.Result.Errors) > 0 {
		fields = append(fields, "result_errors", strings.Join(message.Result.Errors, " | "))
	}
	return fields
}

func buildStreamEventFields(message sdkprotocol.ReceivedMessage) []any {
	if message.Stream == nil {
		return nil
	}
	event := rawMap(message.Stream.Event)
	eventType := strings.TrimSpace(rawString(event["type"]))
	fields := []any{}
	if eventType != "" {
		fields = append(fields, "stream_event_type", eventType)
	}
	switch eventType {
	case "content_block_delta":
		delta := rawMap(event["delta"])
		deltaType := strings.TrimSpace(rawString(delta["type"]))
		if deltaType != "" {
			fields = append(fields, "stream_delta_type", deltaType)
		}
		preview := firstNonEmpty(
			rawString(delta["text"]),
			rawString(delta["thinking"]),
			rawString(delta["partial_json"]),
		)
		if preview != "" {
			fields = append(fields, "stream_preview", truncateForLog(preview))
		}
	case "content_block_start":
		block := rawMap(event["content_block"])
		blockType := strings.TrimSpace(rawString(block["type"]))
		if blockType != "" {
			fields = append(fields, "stream_block_type", blockType)
		}
		preview := firstNonEmpty(rawString(block["text"]), rawString(block["thinking"]), rawString(block["name"]))
		if preview != "" {
			fields = append(fields, "stream_preview", truncateForLog(preview))
		}
	case "message_start", "message_delta", "message_stop", "content_block_stop":
		if preview := truncateForLog(rawJSON(event)); preview != "" {
			fields = append(fields, "stream_payload", preview)
		}
	default:
		if preview := truncateForLog(rawJSON(event)); preview != "" {
			fields = append(fields, "stream_payload", preview)
		}
	}
	return fields
}

func buildToolProgressFields(message sdkprotocol.ReceivedMessage) []any {
	if message.ToolProgress == nil {
		return nil
	}
	return []any{
		"tool_name", strings.TrimSpace(message.ToolProgress.ToolName),
		"tool_use_id", strings.TrimSpace(message.ToolProgress.ToolUseID),
		"task_id", strings.TrimSpace(message.ToolProgress.TaskID),
	}
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
		if message.System.TaskStarted != nil {
			fields = append(
				fields,
				"task_id", strings.TrimSpace(message.System.TaskStarted.TaskID),
				"task_description", truncateForLog(message.System.TaskStarted.Description),
			)
		}
	case "task_progress":
		if message.System.TaskProgress != nil {
			fields = append(
				fields,
				"task_id", strings.TrimSpace(message.System.TaskProgress.TaskID),
				"task_summary", truncateForLog(firstNonEmpty(message.System.TaskProgress.Summary, message.System.TaskProgress.Description)),
				"last_tool_name", strings.TrimSpace(message.System.TaskProgress.LastToolName),
			)
		}
	case "task_notification":
		if message.System.TaskNotification != nil {
			fields = append(
				fields,
				"task_id", strings.TrimSpace(message.System.TaskNotification.TaskID),
				"task_status", strings.TrimSpace(message.System.TaskNotification.Status),
				"task_summary", truncateForLog(message.System.TaskNotification.Summary),
			)
		}
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
		preview = truncateForLog(firstNonEmpty(
			rawString(delta["text"]),
			rawString(delta["thinking"]),
			rawString(delta["partial_json"]),
		))
		if deltaType != "" {
			return appendSummaryPreview(fmt.Sprintf("stream %s(%s)", eventType, deltaType), preview)
		}
	case "content_block_start":
		block := rawMap(event["content_block"])
		blockType := strings.TrimSpace(rawString(block["type"]))
		preview = truncateForLog(firstNonEmpty(
			rawString(block["text"]),
			rawString(block["thinking"]),
			rawString(block["name"]),
		))
		if blockType != "" {
			return appendSummaryPreview(fmt.Sprintf("stream %s(%s)", eventType, blockType), preview)
		}
	default:
		preview = truncateForLog(rawJSON(event))
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
		return appendSummaryPreview("result", truncateForLog(message.Result.Result))
	}
	return appendSummaryPreview("result "+subtype, truncateForLog(message.Result.Result))
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
			return appendSummaryPreview(
				"system task_progress",
				truncateForLog(firstNonEmpty(message.System.TaskProgress.Summary, message.System.TaskProgress.Description)),
			)
		}
	case "task_started":
		if message.System.TaskStarted != nil {
			return appendSummaryPreview("system task_started", truncateForLog(message.System.TaskStarted.Description))
		}
	case "task_notification":
		if message.System.TaskNotification != nil {
			return appendSummaryPreview("system task_notification", truncateForLog(message.System.TaskNotification.Summary))
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
		blockType := normalizeSDKBlockType(block.Type)
		if blockType == "" {
			blockType = "unknown"
		}
		blockTypes = append(blockTypes, blockType)
		switch blockType {
		case "text":
			if text := truncateForLog(block.Text); text != "" {
				previewParts = append(previewParts, text)
			}
		case "thinking":
			if thinking := truncateForLog(block.Thinking); thinking != "" {
				previewParts = append(previewParts, "thinking:"+thinking)
			}
		case "tool_use":
			previewParts = append(previewParts, "tool_use:"+firstNonEmpty(block.Name, block.ID))
		case "tool_result":
			payload := truncateForLog(rawJSON(block.Content))
			if payload == "" {
				payload = block.ToolUseID
			}
			previewParts = append(previewParts, "tool_result:"+payload)
		}
	}
	return blockTypes, truncateForLog(strings.Join(previewParts, " | "))
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

func rawJSON(value any) string {
	if value == nil {
		return ""
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truncateForLog(value string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\n", "\\n"))
	if cleaned == "" {
		return ""
	}
	runes := []rune(cleaned)
	if len(runes) <= sdkMessagePreviewLimit {
		return cleaned
	}
	return string(runes[:sdkMessagePreviewLimit]) + "..."
}
