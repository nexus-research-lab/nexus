package runtime

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
				preview = strings.TrimSpace(rawString(block["name"]))
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
				if toolName := strings.TrimSpace(firstNonEmpty(toolUseBlock.Name, toolUseBlock.ID)); toolName != "" {
					previewParts = append(previewParts, "tool_use:"+toolName)
				}
			}
		}
	}
	return blockTypes, strings.Join(previewParts, " | ")
}
