package tool

import (
	"encoding/json"
	"fmt"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func structuredResult(text string, content map[string]any) sdkmcp.ToolResult {
	return sdkmcp.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		StructuredContent: content,
	}
}

func errorResult(err error) sdkmcp.ToolResult {
	text := "goal tool failed"
	if err != nil {
		text = err.Error()
	}
	return sdkmcp.ToolResult{
		Content: []map[string]any{{
			"type": "text",
			"text": text,
		}},
		IsError: true,
	}
}

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	return nil
}
