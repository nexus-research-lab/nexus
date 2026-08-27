// INPUT: Execution service 与 session-bound runtime context。
// OUTPUT: 顺序稳定的普通 Execution 语义工具 registry；命名 WorkGraph 保存只由隔离专用 registry 暴露。
// POS: Execution command operation 的唯一注册入口。
package operation

import (
	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
)

// BuildAll returns the complete semantic surface. There is deliberately no
// start_work, attempt-state, command-id, or snapshot-revision operation.
func BuildAll(
	svc contract.Service,
	sctx contract.Context,
	workflowServices ...contract.WorkflowService,
) []command.Operation {
	planGuard := &planTransportGuard{attempts: sctx.CommandAttempts}
	operations := []command.Operation{
		getExecution(svc, sctx),
		preparePlanExecution(svc, sctx, planGuard),
		planExecution(svc, sctx),
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
	if len(workflowServices) > 0 {
		if authoring, ok := workflowServices[0].(contract.WorkflowAuthoringService); ok {
			operations = append(operations, BuildWorkGraphAuthoring(authoring, sctx)...)
		}
	}
	return operations
}
