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
		payload["type"] = normalizeBlockType(block.Type)
		mergeNormalizedBlockPayload(payload, block)
		result = append(result, payload)
	}
	return result
}

func cloneBlockPayload(block sdkprotocol.ContentBlock) map[string]any {
	result := map[string]any{}
	if value := strings.TrimSpace(block.Text); value != "" {
		result["text"] = value
	}
	if value := strings.TrimSpace(block.Thinking); value != "" {
		result["thinking"] = value
	}
	if value := strings.TrimSpace(block.Signature); value != "" {
		result["signature"] = value
	}
	if value := strings.TrimSpace(block.ID); value != "" {
		result["id"] = value
	}
	if value := strings.TrimSpace(block.Name); value != "" {
		result["name"] = value
	}
	if len(block.Input) > 0 {
		result["input"] = cloneMap(block.Input)
	}
	if block.Content != nil {
		result["content"] = block.Content
	}
	if value := strings.TrimSpace(block.ToolUseID); value != "" {
		result["tool_use_id"] = value
	}
	if block.IsError {
		result["is_error"] = true
	}
	if value := strings.TrimSpace(block.MimeType); value != "" {
		result["mime_type"] = value
	}
	for key, value := range block.Additional {
		result[key] = value
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
	switch normalizeBlockType(block.Type) {
	case "text":
		payload["text"] = block.Text
	case "thinking":
		payload["thinking"] = block.Thinking
		payload["signature"] = emptyToNil(block.Signature)
	case "tool_use":
		payload["id"] = block.ID
		payload["name"] = block.Name
		payload["input"] = firstNonNilMap(cloneMap(block.Input), map[string]any{})
	case "tool_result":
		payload["tool_use_id"] = block.ToolUseID
		payload["content"] = block.Content
		payload["is_error"] = block.IsError
		payload["mime_type"] = emptyToNil(block.MimeType)
	}
}
