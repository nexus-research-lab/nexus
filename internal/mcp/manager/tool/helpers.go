// INPUT: MCP JSON 参数与 manager 业务返回值。
// OUTPUT: 严格基础类型解析及 JSON/error tool result。
// POS: nexus_manager 无业务逻辑的 transport 辅助层。
package tool

import (
	"encoding/json"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		result, _ := value.Int64()
		return int(result)
	default:
		return 0
	}
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
