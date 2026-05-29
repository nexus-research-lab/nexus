package tool

import (
	"encoding/json"

	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"
)

func jsonResult(payload any) sdktool.ToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": string(data)}},
	}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
