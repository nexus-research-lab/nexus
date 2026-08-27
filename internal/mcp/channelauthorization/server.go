// INPUT: Channel authorization service and server-fixed owner-main DM context.
// OUTPUT: 不含 QR/code/token 材料的单一 Channel 授权 action 工具定义。
// POS: nexus MCP 中的 Channel 授权工具入口。
package channelauthorization

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildTools 创建 Channel 授权 action 工具定义。
func BuildTools(
	svc contract.Service,
	sctx contract.ServerContext,
) []sdktool.Tool {
	return tool.BuildAll(svc, sctx)
}
