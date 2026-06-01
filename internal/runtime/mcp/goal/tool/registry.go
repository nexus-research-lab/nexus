package tool

import (
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"
)

// BuildAll 汇集 Codex Goal 对齐的模型可见工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		getGoal(svc, sctx),
		createGoal(svc, sctx),
		updateGoal(svc, sctx),
	}
}
