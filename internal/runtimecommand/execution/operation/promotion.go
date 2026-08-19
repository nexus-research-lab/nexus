// INPUT: Agent 选择的 persistence reason 与权威 Execution snapshot。
// OUTPUT: 权限/状态允许时绑定 Goal 并升级同轮共享 authority，否则返回结构化拒绝。
// POS: Agent 决定是否需要 Goal；adapter 隐藏身份、版本和绑定细节。
package operation

import (
	"context"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func promoteExecutionToGoal(svc contract.Service, sctx contract.Context) runtimecommand.Operation {
	const operationName = "promote_execution_to_goal"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Bind the current transient Execution to a durable Goal without copying or replacing its Plan. " +
			"Use activation_reason=persistence_requested when the user or system explicitly requested a Goal; otherwise choose an adaptive persistence reason. " +
			"The backend validates objective and criteria presence, authority, user configuration, current state and Goal conflicts.",
		SearchHint:  "promote execution goal persistence boundary recovery wait",
		InputSchema: promoteExecutionSchema(),
		Annotations: &runtimecommand.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(ctx context.Context, input map[string]any, callContext *runtimecommand.CallContext) (runtimecommand.Result, error) {
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
				// Promotion changes WorkGraph-only into Goal+WorkGraph in this
				// physical round. Refresh the actor so the returned execution
				// context and every following Goal/Execution command observe the
				// same exact Goal fence.
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
