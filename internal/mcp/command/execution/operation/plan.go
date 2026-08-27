// INPUT: 完整 Nexus Plan Document、Goal binding intent 与 host-owned durable proposal。
// OUTPUT: sealed proposal、幂等原子 materialization 与 exact-fence 拒绝。
// POS: Execution Plan 准备和提交的统一入口。
package operation

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type planTransportGuard struct {
	emptyPrepare atomic.Uint32
	attempts     *nexusmcp.CommandAttemptState
}

const prepareEmptyAttemptKey = "execution.prepare_plan_execution.empty_input"

func (g *planTransportGuard) incrementPrepare() uint32 {
	if g != nil && g.attempts != nil {
		return g.attempts.Increment(prepareEmptyAttemptKey)
	}
	return g.emptyPrepare.Add(1)
}

func (g *planTransportGuard) resetPrepare() {
	if g != nil && g.attempts != nil {
		g.attempts.Reset(prepareEmptyAttemptKey)
		return
	}
	g.emptyPrepare.Store(0)
}

func preparePlanExecution(
	svc contract.Service,
	sctx contract.Context,
	guards ...*planTransportGuard,
) command.Operation {
	const operationName = "prepare_plan_execution"
	guard := selectPlanTransportGuard(guards)
	return command.Operation{
		Name: operationName,
		Description: "Validate and durably seal one complete Nexus Plan Document v1 without mutating the authoritative Execution, Plan, Goal, Assignment, or Attempt. The sealed proposal itself is durable but non-authoritative until plan_execution commits it. " +
			"Pass one YAML string with nexus_plan: 1; operation create, replan, or replace; and item kind produce, review, verify, or integrate. " +
			"Choose operation only from the current execution inspect result: no current Execution means create, including the first successor Plan after Goal reset or retarget; replan requires the current Execution and preserves its objective boundary; replace requires a current transient Goal-free Execution. " +
			"Every item requires exact keys logical_key, kind, subject, objective, and deliverable. See plan_document schema for the complete parser-backed fields and example, including acceptance_criteria, depends_on, and file:<path>, dir:<path>, or semantic:<key> scopes. " +
			"For a Goal-bound create, finish create_goal first and set goal_binding=current; never launch it in parallel with preparation. current uses only this round's exact Goal authority and is rejected without it. Use goal_binding=none for a Goal-free create. If omitted, create binds current only when exact Goal authority is already present; it never discovers an ambient session Goal. Replan/replace must inherit the existing Execution boundary. Active-Goal create may omit only root objective; every create/replace requires completion_criteria. Change Goal via retarget_goal. " +
			"Unknown keys and aliases are rejected. On success, the host durably binds the sealed proposal; call plan_execution without proposal identifiers. Plan Mode may prepare, but plan_execution is disabled until Plan Mode is exited.",
		SearchHint:  "prepare validate plan document yaml work graph goal binding goal-free proposal depends_on acceptance_criteria",
		InputSchema: preparePlanExecutionSchema(),
		Annotations: &command.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *command.CallContext,
		) (command.Result, error) {
			var parsed preparePlanExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			if strings.TrimSpace(parsed.PlanDocument) == "" {
				return malformedPlanTransportResult(
					operationName,
					"plan_document is required and must be non-empty",
					guard.incrementPrepare(),
				), nil
			}
			guard.resetPrepare()
			prepareCommandID, err := commandID(sctx, callContext, operationName, input, 0)
			if err != nil {
				return transportErrorResult(err), nil
			}
			actor := sctx.Actor()
			proposal, err := svc.PreparePlanExecution(
				ctx,
				actor,
				orchestration.PreparePlanExecutionInput{
					CommandID:    prepareCommandID,
					PlanDocument: parsed.PlanDocument,
					GoalBinding:  parsed.GoalBinding,
				},
			)
			if err != nil {
				if result, ok := planDocumentRejectionResult(operationName, err); ok {
					return result, nil
				}
				var domainErr *orchestration.DomainError
				if errors.As(err, &domainErr) {
					if domainErr.Code == orchestration.ErrorCodeGoalBindingConflict &&
						parsed.GoalBinding == orchestration.PlanGoalBindingCurrent {
						result := orchestration.RejectedResult(nil, err, nil)
						if strings.TrimSpace(actor.GoalID) != "" && actor.GoalObjectiveRevision > 0 {
							result.ContextStatus = "round_refresh_required"
						}
						return mutationResult(result), nil
					}
					if domainErr.Code == orchestration.ErrorCodePlanProposalMismatch {
						return mutationResult(orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
							Domain:    command.DomainExecution,
							Operation: "plan_execution",
							Reason:    "the host already owns the exact active proposal binding; invoke plan_execution with an empty input instead of preparing another proposal",
						}})), nil
					}
					repairReason := "repair the complete Plan Document using the reported validation error"
					if domainErr.Code == orchestration.ErrorCodeNoCurrentExecution {
						repairReason = "execution inspect has no current Execution; seal a complete operation: create Plan, using goal_binding current only with this round's exact Goal authority, otherwise none"
					}
					return mutationResult(orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
						Domain:    command.DomainExecution,
						Operation: operationName,
						Reason:    repairReason,
					}})), nil
				}
				return transportErrorResult(err), nil
			}
			nextActionReason := "invoke plan_execution with no proposal identifiers; the host will atomically materialize this bound sealed Plan"
			message := "Complete Plan proposal is sealed and durably bound; commit it without resending identifiers or the document."
			if sctx.PlanMode {
				nextActionReason = "leave Plan Mode, then invoke plan_execution with no proposal identifiers; the host retains this exact binding across rounds"
				message = "Complete Plan proposal is sealed and durably bound; leave Plan Mode before committing it without identifiers."
			}
			return jsonResult(map[string]any{
				"outcome":                    "prepared",
				"proposal_bound":             true,
				"proposal_status":            proposal.Status,
				"operation":                  proposal.Document.Operation,
				"target_execution_id":        emptyStringToNil(proposal.TargetExecutionID),
				"target_execution_version":   proposal.TargetExecutionVersion,
				"base_plan_id":               emptyStringToNil(proposal.BasePlanID),
				"goal_id":                    emptyStringToNil(proposal.GoalID),
				"goal_objective_revision":    proposal.GoalObjectiveRevision,
				"goal_binding":               planProposalGoalBinding(proposal),
				"objective_source":           planProposalObjectiveSource(proposal),
				"completion_criteria_source": planProposalCompletionCriteriaSource(proposal),
				"item_count":                 len(proposal.Document.Items),
				"message":                    message,
				"next_actions": []orchestration.NextAction{{
					Domain: "execution", Operation: "plan_execution",
					Reason: nextActionReason,
				}},
			}), nil
		},
	}
}

func planProposalGoalBinding(proposal *protocol.ExecutionPlanProposal) orchestration.PlanGoalBindingIntent {
	if proposal == nil {
		return ""
	}
	if proposal.Document.Operation != protocol.ExecutionPlanProposalCreate {
		return orchestration.PlanGoalBindingInherit
	}
	if strings.TrimSpace(proposal.GoalID) != "" && proposal.GoalObjectiveRevision > 0 {
		return orchestration.PlanGoalBindingCurrent
	}
	return orchestration.PlanGoalBindingNone
}

type planDocumentRepairResult struct {
	Outcome          orchestration.MutationOutcome            `json:"outcome"`
	ReasonCode       orchestration.ErrorCode                  `json:"reason_code"`
	Message          string                                   `json:"message"`
	NextActions      []orchestration.NextAction               `json:"next_actions,omitempty"`
	DocumentContract orchestration.PlanDocumentSchemaContract `json:"document_contract"`
}

func planDocumentRejectionResult(
	operationName string,
	err error,
) (command.Result, bool) {
	var documentErr *orchestration.PlanDocumentError
	var domainErr *orchestration.DomainError
	isDocumentError := errors.As(err, &documentErr)
	if errors.As(err, &domainErr) && domainErr.Code == orchestration.ErrorCodePlanDocumentInvalid {
		isDocumentError = true
	}
	if !isDocumentError {
		return command.Result{}, false
	}
	result := orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
		Domain:    command.DomainExecution,
		Operation: operationName,
		Reason:    "rewrite the complete YAML once from document_contract; do not remove fields one by one or invent aliases",
	}})
	return jsonResult(planDocumentRepairResult{
		Outcome:          result.Outcome,
		ReasonCode:       result.ReasonCode,
		Message:          result.Message,
		NextActions:      result.NextActions,
		DocumentContract: orchestration.ExecutionPlanDocumentSchemaContract(),
	}), true
}

func planProposalObjectiveSource(proposal *protocol.ExecutionPlanProposal) string {
	if proposal == nil {
		return ""
	}
	if proposal.Document.Operation == protocol.ExecutionPlanProposalReplan {
		return "execution"
	}
	if proposal.Document.Operation == protocol.ExecutionPlanProposalCreate &&
		strings.TrimSpace(proposal.GoalID) != "" {
		return "goal"
	}
	return "plan_document"
}

func planProposalCompletionCriteriaSource(proposal *protocol.ExecutionPlanProposal) string {
	if proposal != nil && proposal.Document.Operation == protocol.ExecutionPlanProposalReplan {
		return "execution"
	}
	return "plan_document"
}

func malformedPlanTransportResult(operationName, message string, attempt uint32) command.Result {
	actions := []orchestration.NextAction{{
		Domain:    command.DomainExecution,
		Operation: operationName,
		Reason:    "retry once with the required non-empty scalar fields; do not send {} or placeholder values",
	}}
	if attempt > 1 {
		message += "; repeated empty input indicates a command input transport failure, so stop retrying this operation in the current round"
		actions = nil
	}
	return mutationResult(orchestration.RejectedResult(
		nil,
		errors.New(strings.TrimSpace(message)),
		actions,
	))
}

func selectPlanTransportGuard(guards []*planTransportGuard) *planTransportGuard {
	for _, guard := range guards {
		if guard != nil {
			return guard
		}
	}
	return &planTransportGuard{}
}

func emptyStringToNil(value string) any {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return nil
}

func planExecution(
	svc contract.Service,
	sctx contract.Context,
) command.Operation {
	const operationName = "plan_execution"
	return command.Operation{
		Name: operationName,
		Description: "Atomically materialize one sealed immutable Plan proposal. " +
			"First call prepare_plan_execution with the complete Nexus Plan Document; the host durably binds that exact proposal across rounds and process restarts. " +
			"Invoke this operation with no input. Do not send proposal identifiers or reconstruct the WorkGraph. The host-selected proposal remains fenced to the current owner/scope/coordinator/Execution/Goal revision and replays idempotently.",
		SearchHint:  "commit materialize sealed plan proposal execution work graph",
		InputSchema: planExecutionSchema(),
		Annotations: &command.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			_ *command.CallContext,
		) (command.Result, error) {
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
						Domain: command.DomainExecution, Operation: "prepare_plan_execution",
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
) command.Result {
	return mutationResult(orchestration.RejectedResult(nil, &orchestration.DomainError{
		Code: code, Message: message,
	}, []orchestration.NextAction{{
		Domain: command.DomainExecution, Operation: "plan_execution",
		Reason: "retry plan_execution with an empty input so the host can use its exact durable binding",
	}}))
}
