// Package tool 定义 nexus_room MCP 暴露的 Room 通讯工具。
package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/room/contract"
)

// BuildAll 汇集全部 Room 通讯工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		sendDirectedMessage(svc, sctx),
		publishPublicMessage(svc, sctx),
	}
}
