// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：helpers.go
// @Date   ：2026/04/16 21:40:00
// @Author ：leemysw
// 2026/04/16 21:40:00   Create
// =====================================================

package message

import (
	"encoding/json"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func rawString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func normalizeInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func emptyToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneBlockSlice(blocks []map[string]any) []map[string]any {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, cloneMap(block))
	}
	return result
}

func nilIfEmptyMap(source map[string]any) any {
	if len(source) == 0 {
		return nil
	}
	return cloneMap(source)
}

func decodeRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return result
}

func firstNonNilMap(values ...map[string]any) map[string]any {
	for _, value := range values {
		if len(value) > 0 {
			return cloneMap(value)
		}
	}
	return nil
}

func normalizeContentBlocks(blocks []sdkprotocol.ContentBlock) []map[string]any {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		payload := cloneBlockPayload(block)
		if len(payload) == 0 {
			payload = map[string]any{}
		}
		payload["type"] = normalizeBlockType(string(block.Type()))
		mergeNormalizedBlockPayload(payload, block)
		result = append(result, payload)
	}
	return result
}

func cloneBlockPayload(block sdkprotocol.ContentBlock) map[string]any {
	result := cloneMap(block.RawPayload())
	if result == nil {
		result = map[string]any{}
	}
	if value, ok := sdkprotocol.AsTextBlock(block); ok {
		if text := strings.TrimSpace(value.Text); text != "" {
			result["text"] = text
		}
	}
	if value, ok := sdkprotocol.AsThinkingBlock(block); ok {
		if thinking := strings.TrimSpace(value.Thinking); thinking != "" {
			result["thinking"] = thinking
		}
		if signature := strings.TrimSpace(value.Signature); signature != "" {
			result["signature"] = signature
		}
	}
	if value, ok := sdkprotocol.AsToolUseBlock(block); ok {
		if id := strings.TrimSpace(value.ID); id != "" {
			result["id"] = id
		}
		if name := strings.TrimSpace(value.Name); name != "" {
			result["name"] = name
		}
		if input := decodeRawJSON(value.Input); input != nil {
			result["input"] = input
		}
	}
	if value, ok := sdkprotocol.AsToolResultBlock(block); ok {
		if toolUseID := strings.TrimSpace(value.ToolUseID); toolUseID != "" {
			result["tool_use_id"] = toolUseID
		}
		if content := decodeRawJSON(value.Content); content != nil {
			result["content"] = content
		}
		if value.IsError {
			result["is_error"] = true
		}
		if mimeType := strings.TrimSpace(value.MimeType); mimeType != "" {
			result["mime_type"] = mimeType
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeContentBlock(raw any) map[string]any {
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
	}
	if value := normalizeString(result["type"]); value != "" {
		result["type"] = normalizeBlockType(value)
	}
	return result
}

func normalizeBlockType(blockType string) string {
	switch blockType {
	case "server_tool_use":
		return "tool_use"
	case "server_tool_result":
		return "tool_result"
	default:
		return blockType
	}
}

func mergeNormalizedBlockPayload(payload map[string]any, block sdkprotocol.ContentBlock) {
	switch normalizeBlockType(string(block.Type())) {
	case "text":
		if value, ok := sdkprotocol.AsTextBlock(block); ok {
			payload["text"] = value.Text
		}
	case "thinking":
		if value, ok := sdkprotocol.AsThinkingBlock(block); ok {
			payload["thinking"] = value.Thinking
			payload["signature"] = emptyToNil(value.Signature)
		}
	case "tool_use":
		if value, ok := sdkprotocol.AsToolUseBlock(block); ok {
			payload["id"] = value.ID
			payload["name"] = value.Name
			payload["input"] = firstNonNilMap(mapValue(decodeRawJSON(value.Input)), map[string]any{})
		}
	case "tool_result":
		if value, ok := sdkprotocol.AsToolResultBlock(block); ok {
			payload["tool_use_id"] = value.ToolUseID
			payload["content"] = decodeRawJSON(value.Content)
			payload["is_error"] = value.IsError
			payload["mime_type"] = emptyToNil(value.MimeType)
		}
	}
}
