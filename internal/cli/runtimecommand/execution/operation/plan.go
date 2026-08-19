// INPUT: sealed proposal id+digest 与当前 trusted runtime identity。
// OUTPUT: proposal 的幂等原子 materialization、确认后的同轮 Goal authority、恢复回执或 exact-fence 拒绝。
// POS: 模型唯一的权威 Plan 提交入口；不再接收 WorkGraph object/array。
package operation

import (
	"context"
	"strings"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func planExecution(
	svc contract.Service,
	sctx contract.Context,
	guards ...*planTransportGuard,
) runtimecommand.Operation {
	const operationName = "plan_execution"
	guard := selectPlanTransportGuard(guards)
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Atomically materialize one sealed immutable Plan proposal. " +
			"First call prepare_plan_execution with the complete Nexus Plan Document, then pass back exactly its proposal_id and proposal_digest. " +
			"Do not resend, reconstruct, or modify the WorkGraph in this commit call. The proposal survives round and process restarts, is fenced to the current owner/scope/coordinator/Execution/Goal revision, and replays idempotently.",
		SearchHint:  "commit materialize sealed plan proposal execution work graph",
		InputSchema: planExecutionSchema(),
		Annotations: &runtimecommand.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed planExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			if strings.TrimSpace(parsed.ProposalID) == "" ||
				strings.TrimSpace(parsed.ProposalDigest) == "" {
				return malformedPlanTransportResult(
					operationName,
					"proposal_id and proposal_digest are required",
					guard.incrementCommit(),
				), nil
			}
			guard.resetCommit()
			actor := sctx.Actor()
			result, err := svc.MaterializePlanExecution(
				ctx,
				actor,
				orchestration.MaterializePlanExecutionInput{
					ProposalID:     parsed.ProposalID,
					ProposalDigest: parsed.ProposalDigest,
				},
			)
			if err != nil {
				return transportErrorResult(err), nil
			}
			if bindMutationGoalAuthority(sctx, result) {
				actor = sctx.Actor()
			}
			if applyMutationResponsibilityAuthority(sctx, result) {
				actor = sctx.Actor()
			}
			contextActor := actor
			if result.Snapshot != nil &&
				strings.TrimSpace(actor.ExecutionID) != result.Snapshot.Execution.ID {
				contextActor.ExecutionID = ""
				contextActor.WorkBinding = nil
				contextActor.ReviewBinding = nil
			}
			return mutationResult(withFreshExecutionContext(ctx, svc, contextActor, result)), nil
		},
	}
}
