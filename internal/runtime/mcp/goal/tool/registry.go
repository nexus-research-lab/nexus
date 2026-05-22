package tool

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

// BuildAll 汇集 Codex Goal 对齐的模型可见工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdkmcp.Tool {
	return []sdkmcp.Tool{
		getGoal(svc, sctx),
		createGoal(svc, sctx),
		updateGoal(svc, sctx),
	}
}
