// INPUT: Agent SDK 工具参数与 configuration 返回结构。
// OUTPUT: 严格类型参数、RawMessage 和 MCP JSON/error 结果。
// POS: nexus_config 工具的无业务逻辑适配层。
package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func changeRequest(args map[string]any, includeApplyFields bool) (configurationsvc.ChangeRequest, error) {
	input := args["input"]
	if input == nil {
		input = map[string]any{}
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return configurationsvc.ChangeRequest{}, fmt.Errorf("编码 input: %w", err)
	}
	request := configurationsvc.ChangeRequest{
		Domain: stringArg(args, "domain"), Operation: stringArg(args, "operation"),
		Target: stringArg(args, "target"), Input: payload,
	}
	if includeApplyFields {
		request.RequestID = stringArg(args, "request_id")
		request.ExpectedRevision = stringArg(args, "expected_revision")
		request.PlanDigest = stringArg(args, "plan_digest")
	}
	return request, nil
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	value, _ := args[key].(bool)
	return value
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

func stringSliceArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	raw, ok := args[key].([]any)
	if !ok {
		if typed, typedOK := args[key].([]string); typedOK {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func jsonResult(value any) sdktool.ToolResult {
	payload, err := json.Marshal(value)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{Content: []map[string]any{{"type": "text", "text": string(payload)}}}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
