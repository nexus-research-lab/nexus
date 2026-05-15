package tool

import (
	"encoding/json"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func jsonResult(payload any) sdkmcp.ToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return errorResult(err)
	}
	return sdkmcp.ToolResult{
		Content: []map[string]any{{"type": "text", "text": string(data)}},
	}
}

func errorResult(err error) sdkmcp.ToolResult {
	return sdkmcp.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
