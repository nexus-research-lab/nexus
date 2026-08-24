// INPUT: host-owned durable proposal binding、可选 legacy exact pair 与当前 trusted runtime identity。
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
			input map[string]any,
			_ *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed planExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
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
			legacyID := strings.TrimSpace(parsed.ProposalID)
			legacyDigest := strings.TrimSpace(parsed.ProposalDigest)
			if (legacyID == "") != (legacyDigest == "") {
				return planProposalBindingMismatchResult(
					orchestration.ErrorCodeInvalidInput,
					"legacy proposal_id and proposal_digest must either both be omitted or both be present",
				), nil
			}
			if legacyID != "" &&
				(legacyID != proposal.ID || legacyDigest != proposal.ContentDigest) {
				return planProposalBindingMismatchResult(
					orchestration.ErrorCodePlanProposalMismatch,
					"caller-supplied proposal receipt does not match the host-owned active binding",
				), nil
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

func planProposalBindingMismatchResult(
	code orchestration.ErrorCode,
	message string,
) runtimecommand.Result {
	return mutationResult(orchestration.RejectedResult(nil, &orchestration.DomainError{
		Code: code, Message: message,
	}, []orchestration.NextAction{{
		Domain: runtimecommand.DomainExecution, Operation: "plan_execution",
		Reason: "retry plan_execution with an empty input so the host can use its exact durable binding",
	}}))
}
