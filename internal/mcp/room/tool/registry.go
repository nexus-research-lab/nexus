// Package tool 定义 nexus_room MCP 暴露的 Room 通讯工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
)

// BuildAll 汇集全部 Room 通讯工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	if !sctx.PrivateMessagesEnabled {
		return nil
	}
	return []sdktool.Tool{
		sendDirectedMessage(svc, sctx),
		publishPublicMessage(svc, sctx),
	}
}
