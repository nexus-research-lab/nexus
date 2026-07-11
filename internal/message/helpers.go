package message

import (
	"encoding/json"
	"maps"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
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
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	return maps.Clone(source)
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
		// 保留原始 JSON 解析错误，供管道下游（如 PermissionHandler）
		// 检测并拒绝执行，把错误原因反馈给大模型。
		rawStr := strings.TrimSpace(string(raw))
		input := map[string]any{
			"_nexus_parse_error": err.Error(),
		}
		if rawStr != "" {
			input["_nexus_raw_input"] = rawStr
		}
		return input
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
		payload := cloneMap(block.RawPayload())
		if len(payload) == 0 {
			payload = map[string]any{}
		}
		payload["type"] = normalizeBlockType(string(block.Type()))
		mergeNormalizedBlockPayload(payload, block)
		result = append(result, payload)
	}
	return result
}

func normalizeContentBlock(raw any) map[string]any {
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	result := maps.Clone(payload)
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
	case "image":
		if value, ok := sdkprotocol.AsImageBlock(block); ok {
			payload["data"] = value.Data
			payload["mime_type"] = emptyToNil(value.MIMEType)
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
