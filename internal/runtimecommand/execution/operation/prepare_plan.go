// INPUT: 单个完整 Nexus Plan Document string、显式 Goal binding intent 与 trusted current scope/round identity。
// OUTPUT: strict validation 后的 sealed proposal id、Goal boundary、full-fence digest 与 commit 指引。
// POS: Provider 稳定文本传输到 durable non-authoritative proposal 的模型适配入口；模型不能提交 Goal identity。
package operation

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type planTransportGuard struct {
	emptyPrepare atomic.Uint32
	emptyCommit  atomic.Uint32
}

func preparePlanExecution(
	svc contract.Service,
	sctx contract.Context,
	guards ...*planTransportGuard,
) runtimecommand.Operation {
	const operationName = "prepare_plan_execution"
	guard := selectPlanTransportGuard(guards)
	return runtimecommand.Operation{
		Name: operationName,
		Description: "Validate and durably seal one complete Nexus Plan Document v1 without mutating the authoritative Execution, Plan, Goal, Assignment, or Attempt. The sealed proposal itself is durable but non-authoritative until plan_execution commits it. " +
			"Pass one YAML string with nexus_plan: 1; operation create, replan, or replace; and item kind produce, review, verify, or integrate. " +
			"Every item requires exact keys logical_key, kind, subject, objective, and deliverable. See plan_document schema for the complete parser-backed fields and example, including acceptance_criteria, depends_on, and file:<path>, dir:<path>, or semantic:<key> scopes. " +
			"For a Goal-bound create, finish create_goal first and set goal_binding=current; never launch it in parallel with preparation. current uses only this round's exact Goal authority and is rejected without it. Use goal_binding=none for a Goal-free create. If omitted, create binds current only when exact Goal authority is already present; it never discovers an ambient session Goal. Replan/replace must inherit the existing Execution boundary. Active-Goal create may omit only root objective; every create/replace requires completion_criteria. Change Goal via retarget_goal. " +
			"Unknown keys and aliases are rejected. On success, call plan_execution with the returned receipt. Plan Mode may prepare, but plan_execution is disabled until Plan Mode is exited.",
		SearchHint:  "prepare validate plan document yaml work graph goal binding goal-free proposal depends_on acceptance_criteria",
		InputSchema: preparePlanExecutionSchema(),
		Annotations: &runtimecommand.OperationAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *runtimecommand.CallContext,
		) (runtimecommand.Result, error) {
			var parsed preparePlanExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			if strings.TrimSpace(parsed.PlanDocument) == "" {
				return malformedPlanTransportResult(
					operationName,
					"plan_document is required and must be non-empty",
					guard.emptyPrepare.Add(1),
				), nil
			}
			guard.emptyPrepare.Store(0)
			prepareCommandID, err := commandID(sctx, callContext, operationName, input, 0)
			if err != nil {
				return transportErrorResult(err), nil
			}
			proposal, err := svc.PreparePlanExecution(
				ctx,
				sctx.Actor(),
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
					return mutationResult(orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
						Domain:    runtimecommand.DomainExecution,
						Operation: operationName,
						Reason:    "repair the complete Plan Document using the reported validation error",
					}})), nil
				}
				return transportErrorResult(err), nil
			}
			nextActionReason := "pass this exact proposal_id and proposal_digest to atomically materialize the sealed Plan"
			message := "Complete Plan proposal is sealed; commit it without changing the document."
			if sctx.PlanMode {
				nextActionReason = "leave Plan Mode, then pass this exact proposal_id and proposal_digest to materialize the sealed Plan"
				message = "Complete Plan proposal is sealed; leave Plan Mode before committing the unchanged receipt."
			}
			return jsonResult(map[string]any{
				"outcome":                    "prepared",
				"proposal_id":                proposal.ID,
				"proposal_digest":            proposal.ContentDigest,
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
) (runtimecommand.Result, bool) {
	var documentErr *orchestration.PlanDocumentError
	var domainErr *orchestration.DomainError
	isDocumentError := errors.As(err, &documentErr)
	if errors.As(err, &domainErr) && domainErr.Code == orchestration.ErrorCodePlanDocumentInvalid {
		isDocumentError = true
	}
	if !isDocumentError {
		return runtimecommand.Result{}, false
	}
	result := orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
		Domain:    runtimecommand.DomainExecution,
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

func malformedPlanTransportResult(operationName, message string, attempt uint32) runtimecommand.Result {
	actions := []orchestration.NextAction{{
		Domain:    runtimecommand.DomainExecution,
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
