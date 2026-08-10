// INPUT: Channel authorization service and server-fixed owner-main DM context.
// OUTPUT: in-process MCP server whose results never contain QR/code/token material.
// POS: nexus_channel_authorization MCP assembly entry.
package channelauthorization

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func NewServer(
	svc contract.Service,
	sctx contract.ServerContext,
) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(
		contract.ServerName,
		"1.0.0",
		tool.BuildAll(svc, sctx),
	)
}
