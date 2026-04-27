// Package render 负责把 MCP 工具返回值序列化为 MCPToolResult，
// 并为时间字段追加本地化 *_display 展示。
package render

import (
	"encoding/json"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/argx"
)

// JSON 把任意结构体序列化为 MCP text 内容。
func JSON(payload any) agentclient.MCPToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return Error(err)
	}
	return agentclient.MCPToolResult{
		Content: []map[string]any{{"type": "text", "text": string(data)}},
	}
}

// Error 把错误转成 MCP 错误内容（IsError=true）。
func Error(err error) agentclient.MCPToolResult {
	return agentclient.MCPToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}

// timeFields 是会被 DecorateTimes 自动追加 *_display 字段的时间字段。
var timeFields = []string{"next_run_at", "last_run_at", "scheduled_for", "started_at", "finished_at"}

// DecorateTimes 为返回 payload 附加本地时区字符串字段（*_display）。
func DecorateTimes(payload any, timezoneHint string) any {
	raw, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return payload
	}
	return decorateNode(decoded, argx.FirstNonEmpty(timezoneHint, "Asia/Shanghai"))
}

func decorateNode(node any, tz string) any {
	switch v := node.(type) {
	case []any:
		for i, item := range v {
			v[i] = decorateNode(item, tz)
		}
		return v
	case map[string]any:
		resolvedTz := tz
		if schedule, ok := v["schedule"].(map[string]any); ok {
			if s := strings.TrimSpace(argx.StringOf(schedule["timezone"])); s != "" {
				resolvedTz = s
			}
			v["schedule"] = decorateNode(schedule, resolvedTz)
		}
		for _, key := range timeFields {
			if s := strings.TrimSpace(argx.StringOf(v[key])); s != "" {
				if display, ok := formatDisplayTime(s, resolvedTz); ok {
					v[key+"_display"] = display
				}
			}
		}
		return v
	default:
		return node
	}
}

func formatDisplayTime(value, tz string) (string, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return "", false
		}
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", false
	}
	return parsed.In(loc).Format("2006-01-02 15:04:05 MST"), true
}
