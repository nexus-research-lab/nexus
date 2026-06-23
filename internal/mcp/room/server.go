// Package roommcp 提供 Room 通讯内建 MCP server 入口。
package roommcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/room/tool"
)

// NewServer 根据当前 Room 成员上下文构建 nexus_room MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
