// INPUT: host-owned durable proposal binding 与当前 trusted runtime identity；模型参数为空。
// OUTPUT: bound proposal 的幂等原子 materialization、确认后的同轮 Goal authority、恢复回执或 exact-fence 拒绝。
// POS: 模型唯一的权威 Plan 提交入口；正常调用零输入，caller 不能选择 proposal 或重发 WorkGraph。
package operation

import (
	"context"
	"errors"
	"strings"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func planExecution(
	svc contract.Service,
	sctx contract.Context,
) runtimecommand.Operation {
	const operationName = "plan_execution"
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Atomically materialize one sealed immutable Plan proposal. " +
			"First call prepare_plan_execution with the complete Nexus Plan Document; the host durably binds that exact proposal across rounds and process restarts. " +
			"Invoke this operation with no input. Do not send proposal identifiers or reconstruct the WorkGraph. The host-selected proposal remains fenced to the current owner/scope/coordinator/Execution/Goal revision and replays idempotently.",
		SearchHint:  "commit materialize sealed plan proposal execution work graph",
		InputSchema: planExecutionSchema(),
		Annotations: &runtimecommand.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			_ map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			actor := sctx.Actor()
			proposal, err := svc.ResolvePlanExecutionProposal(ctx, actor)
			if err != nil {
				var domainErr *orchestration.DomainError
				if errors.As(err, &domainErr) {
					return mutationResult(orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
						Domain: runtimecommand.DomainExecution, Operation: "prepare_plan_execution",
						Reason: "prepare one complete Plan Document so the host can establish an exact durable proposal binding",
					}})), nil
				}
				return transportErrorResult(err), nil
			}
			if proposal == nil || strings.TrimSpace(proposal.ID) == "" ||
				strings.TrimSpace(proposal.ContentDigest) == "" {
				return transportErrorResult(errors.New("resolved Plan proposal binding is incomplete")), nil
			}
			result, err := svc.MaterializePlanExecution(
				ctx,
				actor,
				orchestration.MaterializePlanExecutionInput{
					ProposalID:     proposal.ID,
					ProposalDigest: proposal.ContentDigest,
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
