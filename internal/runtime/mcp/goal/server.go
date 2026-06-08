package goalmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/tool"
)

// NewServer 根据当前会话上下文构建 nexus_goal MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
