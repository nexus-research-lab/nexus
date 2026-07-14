// INPUT: Goal service 与 MCP server context。
// OUTPUT: 模型可见的完整 Goal 工具集合。
// POS: Goal MCP 工具注册入口。
package tool

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildAll 汇集 Codex Goal 对齐的模型可见工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		getGoal(svc, sctx),
		createGoal(svc, sctx),
		retargetGoal(svc, sctx),
		updateGoal(svc, sctx),
	}
}
