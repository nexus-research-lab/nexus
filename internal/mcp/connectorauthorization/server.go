// INPUT: Connector authorization 服务与 server 固化的 owner-main DM 上下文。
// OUTPUT: 只包含 start/status/cancel 的进程内 MCP server。
// POS: nexus_connector_auth 顶层装配入口。
package connectorauthorization

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// NewServer 创建专用 Connector 授权 MCP server。
func NewServer(
	svc contract.Service,
	sctx contract.ServerContext,
) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(
		contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx),
	)
}
