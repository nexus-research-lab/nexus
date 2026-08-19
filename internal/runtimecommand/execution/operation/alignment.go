// INPUT: Agent 选择发起的 Execution 目标对齐报告与 trusted runtime context。
// OUTPUT: 一个可见但不驱动路由的 Gate NodeRun mutation result。
// POS: 可选 objective alignment 观测入口；不是 Goal lifecycle，也不是自动 loop scheduler。
package operation

import (
	"context"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func auditExecutionAlignment(
	svc contract.Service,
	sctx contract.Context,
) runtimecommand.Operation {
	const operationName = "audit_execution_alignment"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Record an optional three-state evidence audit of the current Execution objective against its authoritative completion criteria as a visible Gate. " +
			"It is valid only while that Execution is current and never transitions the Execution, starts a Goal, retries work or selects the next route. " +
			"Do not use it as the completion audit for a Goal+WorkGraph flow and do not call it after the final accepted review makes the Execution terminal; " +
			"that mixed flow continues in the Goal domain with audit_objective_alignment.",
		SearchHint:  "execution objective alignment gate checkpoint evidence loop",
		InputSchema: auditExecutionAlignmentSchema(),
		Annotations: &runtimecommand.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed auditExecutionAlignmentInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(
				ctx,
				svc,
				sctx,
				actor,
				parsed.ExecutionID,
				operationName,
				input,
				callContext,
			)
			if result != nil {
				return *result, nil
			}
			response, err := svc.AuditExecutionAlignment(
				ctx,
				actor,
				orchestration.AuditExecutionAlignmentInput{
					ExecutionID:      snapshot.Execution.ID,
					SnapshotRevision: snapshot.Execution.Version,
					CommandID:        command,
					Report:           parsed.report(),
				},
			)
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}
