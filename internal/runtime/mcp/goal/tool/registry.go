package tool

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

// BuildAll 汇集全部 Goal 工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdkmcp.Tool {
	return []sdkmcp.Tool{
		getCurrent(svc, sctx),
		complete(svc, sctx),
		block(svc, sctx),
	}
}
