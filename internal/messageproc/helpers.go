// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：helpers.go
// @Date   ：2026/04/16 21:40:00
// @Author ：leemysw
// 2026/04/16 21:40:00   Create
// =====================================================

package messageproc

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
	if block == nil {
		return nil
	}
	payload := block.RawPayload()
	if len(payload) == 0 {
		return nil
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
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
	switch typed, ok := sdkprotocol.AsTextBlock(block); {
	case ok:
		payload["text"] = typed.Text
		return
	}
	switch typed, ok := sdkprotocol.AsThinkingBlock(block); {
	case ok:
		payload["thinking"] = typed.Thinking
		payload["signature"] = emptyToNil(typed.Signature)
		return
	}
	switch typed, ok := sdkprotocol.AsToolUseBlock(block); {
	case ok:
		payload["id"] = typed.ID
		payload["name"] = typed.Name
		payload["input"] = firstNonNilMap(typed.InputMap(), map[string]any{})
		return
	}
	switch typed, ok := sdkprotocol.AsToolResultBlock(block); {
	case ok:
		payload["tool_use_id"] = typed.ToolUseID
		payload["content"] = decodeRawJSON(typed.Content)
		payload["is_error"] = typed.IsError
		payload["mime_type"] = emptyToNil(typed.MimeType)
		return
	}
}
