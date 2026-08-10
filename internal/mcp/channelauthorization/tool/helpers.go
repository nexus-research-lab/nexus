// INPUT: strict MCP args and safe service values.
// OUTPUT: trimmed strings and JSON/error results.
// POS: transport-only helpers with no authorization or Channel logic.
package tool

import (
	"encoding/json"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func jsonResult(value any) sdktool.ToolResult {
	payload, err := json.Marshal(value)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": string(payload)}},
	}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
