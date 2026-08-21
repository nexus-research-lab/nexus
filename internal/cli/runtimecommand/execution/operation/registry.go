// INPUT: Execution service 与 session-bound runtime context。
// OUTPUT: 顺序稳定的 Execution 与命名 WorkGraph 保存语义工具 registry。
// POS: Execution command operation 的唯一注册入口。
package operation

import (
	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
)

// BuildAll returns the complete semantic surface. There is deliberately no
// start_work, attempt-state, command-id, or snapshot-revision operation.
func BuildAll(
	svc contract.Service,
	sctx contract.Context,
	workflowServices ...contract.WorkflowService,
) []runtimecommand.Operation {
	planGuard := &planTransportGuard{attempts: sctx.CommandAttempts}
	operations := []runtimecommand.Operation{
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
	if len(workflowServices) > 0 && workflowServices[0] != nil {
		operations = append(operations, distillWorkGraphWorkflow(workflowServices[0], sctx))
	}
	return operations
}
