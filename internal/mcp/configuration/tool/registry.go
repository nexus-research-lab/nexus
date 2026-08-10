// INPUT: configuration 服务与可信 Agent/DM/Room runtime 上下文。
// OUTPUT: 配置发现、预检、应用与审计工具集合。
// POS: nexus_config MCP 工具注册表。
package tool

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

// BuildAll 返回全部配置控制工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		inspect(svc, sctx),
		plan(svc, sctx),
		apply(svc, sctx),
		history(svc, sctx),
	}
}
