// INPUT: Execution service 与 session-bound runtime context。
// OUTPUT: 顺序稳定、固定十个语义工具的 registry。
// POS: nexus_execution 模型工具唯一注册入口。
package tool

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildAll returns the complete semantic surface. There is deliberately no
// start_work, attempt-state, command-id, or snapshot-revision tool.
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		getExecution(svc, sctx),
		planExecution(svc, sctx),
		abandonExecution(svc, sctx),
		assignWork(svc, sctx),
		submitWork(svc, sctx),
		reviewWork(svc, sctx),
		blockWork(svc, sctx),
		resumeWork(svc, sctx),
		takeOverWork(svc, sctx),
		promoteExecutionToGoal(svc, sctx),
	}
}
