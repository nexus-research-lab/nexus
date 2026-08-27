// INPUT: Execution 目标对齐报告、persistence reason 与 trusted runtime context。
// OUTPUT: 可见 Gate 结果，或绑定 Goal 后的同轮共享 authority。
// POS: Execution 与 Goal 边界的审计和晋升入口。
package operation

import (
	"context"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func auditExecutionAlignment(
	svc contract.Service,
	sctx contract.Context,
) command.Operation {
	const operationName = "audit_execution_alignment"
	return command.Operation{
		Name: operationName,
		Description: "Record an optional three-state evidence audit of the current Execution objective against its authoritative completion criteria as a visible Gate. " +
			"It is valid only while that Execution is current and never transitions the Execution, starts a Goal, retries work or selects the next route. " +
			"Do not use it as the completion audit for a Goal+WorkGraph flow and do not call it after the final accepted review makes the Execution terminal; " +
			"that mixed flow continues in the Goal domain with audit_objective_alignment.",
		SearchHint:  "execution objective alignment gate checkpoint evidence loop",
		InputSchema: auditExecutionAlignmentSchema(),
		Annotations: &command.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *command.CallContext,
		) (command.Result, error) {
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

func promoteExecutionToGoal(svc contract.Service, sctx contract.Context) command.Operation {
	const operationName = "promote_execution_to_goal"
	return command.Operation{
		Name: operationName,
		Description: "Bind the current transient Execution to a durable Goal without copying or replacing its Plan. " +
			"Use activation_reason=persistence_requested when the user or system explicitly requested a Goal; otherwise choose an adaptive persistence reason. " +
			"The backend validates objective and criteria presence, authority, user configuration, current state and Goal conflicts.",
		SearchHint:  "promote execution goal persistence boundary recovery wait",
		InputSchema: promoteExecutionSchema(),
		Annotations: &command.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *command.CallContext) (command.Result, error) {
			var parsed promoteExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			snapshot, command, result := mutationEnvelope(ctx, svc, sctx, actor, parsed.ExecutionID, operationName, input, callContext)
			if result != nil {
				return *result, nil
			}
			response, err := svc.PromoteExecutionToGoal(ctx, actor, orchestration.PromoteExecutionToGoalInput{
				ExecutionID:       snapshot.Execution.ID,
				SnapshotRevision:  snapshot.Execution.Version,
				CommandID:         command,
				ObjectiveProposal: parsed.ObjectiveProposal,
				ActivationReason:  parsed.ActivationReason,
			})
			if err == nil && bindMutationGoalAuthority(sctx, response) {
				// 晋升在当前 physical round 内改变 authority，后续投影必须读取同一精确 Goal fence。
				actor = sctx.Actor()
			}
			return serviceMutation(ctx, svc, actor, response, err), nil
		},
	}
}

func bindMutationGoalAuthority(
	sctx contract.Context,
	result orchestration.MutationResult,
) bool {
	if result.Outcome != orchestration.MutationApplied &&
		result.Outcome != orchestration.MutationNoOp ||
		result.GoalAuthority == nil {
		return false
	}
	receipt := result.GoalAuthority
	if receipt.GoalID == "" || receipt.ObjectiveRevision <= 0 ||
		receipt.ExecutionID == "" {
		return false
	}
	if sctx.ResponsibilityAuthority != nil {
		return sctx.ResponsibilityAuthority.ConfirmGoalExecution(
			receipt.GoalID,
			receipt.ObjectiveRevision,
			receipt.ExecutionID,
		)
	}
	return sctx.GoalAuthority.Bind(
		receipt.GoalID,
		receipt.ObjectiveRevision,
		receipt.ExecutionID,
	)
}
