// INPUT: nexusmanager 服务与 server 固化的 runtime 上下文。
// OUTPUT: 按当前权限裁剪工具列表的 in-process nexus_manager MCP server。
// POS: nexus_manager MCP 装配入口。
package manager

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/manager/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// NewServer 创建受控 Nexus 资源管理 MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
