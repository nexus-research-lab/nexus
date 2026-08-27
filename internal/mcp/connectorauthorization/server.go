// INPUT: Connector authorization 服务与 server 固化的 owner-main DM 上下文。
// OUTPUT: 包含 start/status/cancel action 的单一 Connector 授权工具定义。
// POS: nexus MCP 中的 Connector 授权工具入口。
package connectorauthorization

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildTools 创建 Connector 授权 action 工具定义。
func BuildTools(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	return tool.BuildAll(svc, sctx)
}
