// Package sdktool 把 Nexus 内部 MCP 工具描述装配成 Agent SDK tool。
package sdktool

import (
	"context"
	"errors"

	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
)

// Tool 表示 Nexus 内部 MCP 工具定义。
type Tool struct {
	Name        string
	Description string
	SearchHint  string
	AlwaysLoad  bool
	InputSchema map[string]any
	Annotations *ToolAnnotations
	Handler     func(context.Context, map[string]any) (ToolResult, error)
}

// ToolResult 表示 MCP 工具调用结果。
type ToolResult = sdktools.Result

// ToolAnnotations 表示工具元数据。
type ToolAnnotations = sdktools.Annotations

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
			func(ctx context.Context, input map[string]any, _ *sdktools.Context) (sdktools.Result, error) {
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
