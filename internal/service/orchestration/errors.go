// INPUT: Execution Orchestration 的服务端拒绝原因。
// OUTPUT: 可被 MCP、Hook、Room admission 与测试稳定识别的错误码。
// POS: 领域错误边界；不得把 SQL/driver 文本直接暴露给模型。
package orchestration

import (
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrorCode 是模型可恢复 mutation 使用的稳定拒绝码。
type ErrorCode string

const (
	ErrorCodeInvalidInput            ErrorCode = "invalid_input"
	ErrorCodePlanDocumentInvalid     ErrorCode = "plan_document_invalid"
	ErrorCodePlanProposalNotFound    ErrorCode = "plan_proposal_not_found"
	ErrorCodePlanProposalBinding     ErrorCode = "plan_proposal_binding_missing"
	ErrorCodePlanProposalMismatch    ErrorCode = "plan_proposal_binding_mismatch"
	ErrorCodePlanProposalDigest      ErrorCode = "plan_proposal_digest_mismatch"
	ErrorCodePlanProposalStale       ErrorCode = "plan_proposal_stale"
	ErrorCodePlanProposalBlocked     ErrorCode = "plan_proposal_blocked"
	ErrorCodePlanItemsEmpty          ErrorCode = "plan_items_empty"
	ErrorCodeDuplicateLogicalKey     ErrorCode = "duplicate_logical_key"
	ErrorCodeUnknownDependency       ErrorCode = "unknown_dependency"
	ErrorCodeDependencyCycle         ErrorCode = "dependency_cycle"
	ErrorCodeTerminalWorkMissing     ErrorCode = "terminal_work_missing"
	ErrorCodeOutputScopeConflict     ErrorCode = "output_scope_conflict"
	ErrorCodeAcceptanceCriteriaEmpty ErrorCode = "acceptance_criteria_empty"
	ErrorCodeCompletionCriteriaEmpty ErrorCode = "completion_criteria_empty"
	ErrorCodeProjectionLimitExceeded ErrorCode = "projection_limit_exceeded"
	ErrorCodeStaleExecution          ErrorCode = "stale_execution"
	ErrorCodeWrongOwner              ErrorCode = "wrong_owner"
	ErrorCodeWrongReviewer           ErrorCode = "wrong_reviewer"
	ErrorCodeDependencyNotAccepted   ErrorCode = "dependency_not_accepted"
	ErrorCodeDuplicateAssignment     ErrorCode = "duplicate_assignment"
	ErrorCodeDuplicateAttempt        ErrorCode = "duplicate_attempt"
	ErrorCodeAssignmentTargetInvalid ErrorCode = "assignment_target_invalid"
	ErrorCodeRoomReviewerRequired    ErrorCode = "room_independent_reviewer_required"
	ErrorCodeNoCurrentExecution      ErrorCode = "no_current_execution"
	ErrorCodeNoDelegableAssignment   ErrorCode = "no_delegable_assignment"
	ErrorCodeAmbiguousAssignment     ErrorCode = "ambiguous_assignment"
	ErrorCodeSubagentAlreadyActive   ErrorCode = "subagent_already_active"
	ErrorCodeSubagentBindingMissing  ErrorCode = "subagent_binding_missing"
	ErrorCodeWorkBindingMismatch     ErrorCode = "work_binding_mismatch"
	ErrorCodeReviewBindingRequired   ErrorCode = "review_binding_required"
	ErrorCodeConversationOnly        ErrorCode = "conversation_only"
	ErrorCodePlanMode                ErrorCode = "plan_mode"
	ErrorCodeExecutionTerminal       ErrorCode = "execution_terminal"
	ErrorCodeObjectiveChangeReplace  ErrorCode = "execution_objective_change_requires_replace"
	ErrorCodeGoalRetargetRequired    ErrorCode = "goal_retarget_required"
	ErrorCodeCompletionBlocked       ErrorCode = "completion_blocked"
	ErrorCodeGoalObjectiveConflict   ErrorCode = "goal_objective_conflict"
	ErrorCodeGoalScopeConflict       ErrorCode = "goal_scope_conflict"
	ErrorCodeGoalBindingConflict     ErrorCode = "goal_binding_conflict"
)

// DomainError 保留稳定 reason code 和面向调用者的具体修复上下文。
type DomainError struct {
	Code        ErrorCode
	Message     string
	WorkItemKey string
	RelatedKey  string
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	if e.WorkItemKey != "" {
		return fmt.Sprintf("%s: work item %s: %s", e.Code, e.WorkItemKey, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newDomainError(code ErrorCode, message string, workItemKey string, relatedKey string) error {
	return &DomainError{
		Code:        code,
		Message:     message,
		WorkItemKey: workItemKey,
		RelatedKey:  relatedKey,
	}
}

func newProjectionLimitError(field string, count int, workItemKey string) error {
	err := protocol.ValidateExecutionProjectionLimit(field, count)
	if err == nil {
		return nil
	}
	message := err.Error()
	if typed, ok := err.(*protocol.ExecutionProjectionLimitError); ok {
		message = fmt.Sprintf(
			"%s has %d items; maximum is %d",
			typed.Field,
			typed.Count,
			typed.Limit,
		)
	}
	return newDomainError(
		ErrorCodeProjectionLimitExceeded,
		message,
		workItemKey,
		field,
	)
}
