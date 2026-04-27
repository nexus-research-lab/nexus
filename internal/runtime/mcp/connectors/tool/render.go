package tool

import (
	"encoding/json"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

func jsonResult(payload any) agentclient.MCPToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return errorResult(err)
	}
	return agentclient.MCPToolResult{
		Content: []map[string]any{{"type": "text", "text": string(data)}},
	}
}

func errorResult(err error) agentclient.MCPToolResult {
	return agentclient.MCPToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
