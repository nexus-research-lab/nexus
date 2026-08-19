// INPUT: Execution service 与 session-bound runtime context。
// OUTPUT: 顺序稳定、固定十二个语义工具的 registry。
// POS: Execution command operation 的唯一注册入口。
package operation

import (
	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
)

// BuildAll returns the complete semantic surface. There is deliberately no
// start_work, attempt-state, command-id, or snapshot-revision operation.
func BuildAll(svc contract.Service, sctx contract.Context) []runtimecommand.Operation {
	planGuard := &planTransportGuard{attempts: sctx.CommandAttempts}
	return []runtimecommand.Operation{
		getExecution(svc, sctx),
		preparePlanExecution(svc, sctx, planGuard),
		planExecution(svc, sctx, planGuard),
		abandonExecution(svc, sctx),
		assignWork(svc, sctx),
		submitWork(svc, sctx),
		reviewWork(svc, sctx),
		blockWork(svc, sctx),
		resumeWork(svc, sctx),
		takeOverWork(svc, sctx),
		auditExecutionAlignment(svc, sctx),
		promoteExecutionToGoal(svc, sctx),
	}
}
