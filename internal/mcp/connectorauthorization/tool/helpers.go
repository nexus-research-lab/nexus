// INPUT: MCP 参数和 service 公开投影。
// OUTPUT: 严格字符串 map、Authorization request/ref 与 JSON/error result。
// POS: Nexus Connector 授权工具的无业务逻辑适配层。
package tool

import (
	"encoding/json"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func startRequest(args map[string]any) connectorsvc.AuthorizationStartRequest {
	return connectorsvc.AuthorizationStartRequest{
		RequestID:   stringArg(args, "request_id"),
		ConnectorID: stringArg(args, "connector_id"),
		Method:      stringArg(args, "method"),
		DeviceMode: connectorsvc.DeviceAuthStartMode(
			stringArg(args, "device_mode"),
		),
		Extras: stringMapArg(args, "extras"),
	}
}

func flowRef(args map[string]any) connectorsvc.AuthorizationFlowRef {
	return connectorsvc.AuthorizationFlowRef{
		FlowID:      stringArg(args, "flow_id"),
		ConnectorID: stringArg(args, "connector_id"),
	}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func stringMapArg(args map[string]any, key string) map[string]string {
	if args == nil || args[key] == nil {
		return nil
	}
	if typed, ok := args[key].(map[string]string); ok {
		result := make(map[string]string, len(typed))
		for itemKey, value := range typed {
			result[itemKey] = value
		}
		return result
	}
	raw, ok := args[key].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(raw))
	for itemKey, value := range raw {
		if text, textOK := value.(string); textOK {
			result[itemKey] = text
		}
	}
	return result
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
