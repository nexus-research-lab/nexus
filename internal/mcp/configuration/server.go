// INPUT: configuration 服务与服务端注入的当前交互式 Agent/DM/Room runtime 上下文。
// OUTPUT: nexus_config 进程内 MCP server。
// POS: 可信 runtime 身份进入 configuration MCP 的顶层装配入口。
package configurationmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/tool"
)

// NewServer 创建配置控制 MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
