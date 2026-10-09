// Package sdktool 把 Nexus 内部 MCP 工具描述装配成 Agent SDK tool。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package sdktool

import (
	"context"
	"encoding/json"
	"errors"

	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
)

// Tool 表示 Nexus 内部 MCP 工具定义。
type Tool struct {
	Name           string
	Description    string
	SearchHint     string
	AlwaysLoad     bool
	InputSchema    map[string]any
	Annotations    *ToolAnnotations
	Handler        func(context.Context, map[string]any) (ToolResult, error)
	ContextHandler func(context.Context, map[string]any, *CallContext) (ToolResult, error)
}

// ToolResult 表示 MCP 工具调用结果。
type ToolResult = sdktools.Result

// ErrorResult 把错误投影为单段文本的失败结果。
func ErrorResult(err error) ToolResult {
	return ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}

// JSONResult 把值编码为单段 JSON 文本；编码失败时返回 ErrorResult。
func JSONResult(value any) ToolResult {
	payload, err := json.Marshal(value)
	if err != nil {
		return ErrorResult(err)
	}
	return ToolResult{Content: []map[string]any{{"type": "text", "text": string(payload)}}}
}

// StructuredJSONResult 在 JSONResult 之外同时返回 StructuredContent。
func StructuredJSONResult(value map[string]any) ToolResult {
	result := JSONResult(value)
	if !result.IsError {
		result.StructuredContent = value
	}
	return result
}

// ToolAnnotations 表示工具元数据。
type ToolAnnotations = sdktools.Annotations

// CallContext 承载 bridge 可提供的 tool_use、SDK session、round 与来源 identity。
// Bridge 从 MCP params._meta 传递真实 tool-use identity；缺省字段保持为空。
// 依赖调用身份的副作用必须拒绝缺失，不能用正文哈希冒充真实调用 ID。
type CallContext = sdktools.Context

// SimpleSDKMCPServer 表示 SDK 进程内 MCP server。
type SimpleSDKMCPServer = sdktools.SimpleSDKMCPServer

// NewSimpleSDKMCPServer 创建 SDK 进程内 MCP server。
func NewSimpleSDKMCPServer(name string, version string, definitions []Tool) *SimpleSDKMCPServer {
	tools := make([]sdktools.Tool, 0, len(definitions))
	for _, definition := range definitions {
		definition := definition
		options := make([]sdktools.ToolOption, 0, 3)
		if definition.SearchHint != "" {
			options = append(options, sdktools.WithSearchHint(definition.SearchHint))
		}
		if definition.AlwaysLoad {
			options = append(options, sdktools.WithAlwaysLoad(true))
		}
		if definition.Annotations != nil {
			options = append(options, sdktools.WithAnnotations(*definition.Annotations))
		}
		tools = append(tools, sdktools.New(
			definition.Name,
			definition.Description,
			definition.InputSchema,
			func(ctx context.Context, input map[string]any, callContext *sdktools.Context) (sdktools.Result, error) {
				if definition.ContextHandler != nil {
					return definition.ContextHandler(ctx, input, callContext)
				}
				if definition.Handler == nil {
					return sdktools.Result{}, errors.New("sdktool: tool handler is nil")
				}
				return definition.Handler(ctx, input)
			},
			options...,
		))
	}
	return sdktools.CreateSDKMCPServer(sdktools.SDKMCPServerOptions{
		Name:    name,
		Version: version,
		Tools:   tools,
	})
}
