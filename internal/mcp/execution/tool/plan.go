// INPUT: sealed proposal id+digest 与当前 trusted runtime identity。
// OUTPUT: proposal 的幂等原子 materialization、确认后的同轮 Goal authority、恢复回执或 exact-fence 拒绝。
// POS: 模型唯一的权威 Plan 提交入口；不再接收 WorkGraph object/array。
package tool

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func planExecution(
	svc contract.Service,
	sctx contract.ServerContext,
	guards ...*planTransportGuard,
) sdktool.Tool {
	const toolName = "plan_execution"
	guard := selectPlanTransportGuard(guards)
	return sdktool.Tool{
		Name: toolName,
		Description: "Atomically materialize one sealed immutable Plan proposal. " +
			"First call prepare_plan_execution with the complete Nexus Plan Document, then pass back exactly its proposal_id and proposal_digest. " +
			"Do not resend, reconstruct, or modify the WorkGraph in this commit call. The proposal survives round and process restarts, is fenced to the current owner/scope/coordinator/Execution/Goal revision, and replays idempotently.",
		SearchHint:  "commit materialize sealed plan proposal execution work graph",
		InputSchema: planExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed planExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			if strings.TrimSpace(parsed.ProposalID) == "" ||
				strings.TrimSpace(parsed.ProposalDigest) == "" {
				return malformedPlanTransportResult(
					toolName,
					"proposal_id and proposal_digest are required",
					guard.emptyCommit.Add(1),
				), nil
			}
			guard.emptyCommit.Store(0)
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
