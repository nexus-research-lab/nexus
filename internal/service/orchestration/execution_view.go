// INPUT: owner/session 只读查询与当前或最近一次 ExecutionSnapshot。
// OUTPUT: 去除控制面 identity、按 Plan position 排序并派生交付阶段的 protocol.ExecutionView。
// POS: Execution 状态机到 HTTP/DM/Room WorkGraph UI 的唯一展示投影。
package orchestration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type latestExecutionRepository interface {
	FindLatest(context.Context, string, string) (*protocol.Execution, error)
}

// GetLatestView 返回 session 当前 Execution；没有未终结 Execution 时保留最近一次
// terminal 结果，使用户能看到完成、取消或失败结论并自行收起。
func (s *Service) GetLatestView(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.ExecutionView, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return nil, domainError(ErrorCodeInvalidInput, "owner and session_key are required")
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	execution, err := s.repository.FindCurrent(ctx, ownerUserID, sessionKey)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		latestRepository, ok := s.repository.(latestExecutionRepository)
		if !ok {
			return nil, nil
		}
		execution, err = latestRepository.FindLatest(ctx, ownerUserID, sessionKey)
		if err != nil || execution == nil {
			return nil, err
		}
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil {
		return nil, err
	}
	if snapshot.Execution.OwnerUserID != ownerUserID ||
		snapshot.Execution.SessionKey != sessionKey {
		return nil, domainError(ErrorCodeWrongOwner, "Execution is outside the requested owner/session")
	}
	return ProjectExecutionView(snapshot), nil
}

// ProjectExecutionView 把权威 snapshot 投影成稳定且安全的 UI 读取模型。
func ProjectExecutionView(snapshot *protocol.ExecutionSnapshot) *protocol.ExecutionView {
	if snapshot == nil || strings.TrimSpace(snapshot.Execution.ID) == "" {
		return nil
	}
	execution := snapshot.Execution
	result := &protocol.ExecutionView{
		ID:                    execution.ID,
		SessionKey:            execution.SessionKey,
		ScopeKind:             execution.ScopeKind,
		RoomID:                execution.RoomID,
		ConversationID:        execution.ConversationID,
		CoordinatorAgentID:    execution.CoordinatorAgentID,
		Objective:             execution.Objective,
		CompletionCriteria:    slices.Clone(execution.CompletionCriteria),
		GoalID:                execution.GoalID,
		GoalObjectiveRevision: execution.GoalObjectiveRevision,
		Status:                execution.Status,
		Version:               execution.Version,
		CompletionBlockers:    slices.Clone(snapshot.CompletionBlockers),
		CreatedAt:             execution.CreatedAt,
		UpdatedAt:             execution.UpdatedAt,
		CompletedAt:           execution.CompletedAt,
	}
	if snapshot.Plan == nil {
		return result
	}
	result.Plan = &protocol.ExecutionPlanView{
		ID:             snapshot.Plan.ID,
		Revision:       snapshot.Plan.Revision,
		Status:         snapshot.Plan.Status,
		RevisionReason: snapshot.Plan.RevisionReason,
		CreatedAt:      snapshot.Plan.CreatedAt,
		ActivatedAt:    snapshot.Plan.ActivatedAt,
	}

	view := newExecutionContextView(snapshot)
	planItems := slices.Clone(snapshot.PlanItems)
	slices.SortFunc(planItems, func(left, right protocol.ExecutionPlanItem) int {
		if left.Position != right.Position {
			return left.Position - right.Position
		}
		return strings.Compare(left.WorkItemID, right.WorkItemID)
	})
	result.WorkItems = make([]protocol.ExecutionWorkItemView, 0, len(planItems))
	for _, planItem := range planItems {
		workItem, workExists := view.workItems[planItem.WorkItemID]
		spec, specExists := view.specs[planItem.SpecID]
		if !workExists || !specExists {
			continue
		}
		item := projectExecutionWorkItemView(snapshot, view, planItem, workItem, spec)
		result.WorkItems = append(result.WorkItems, item)
		incrementExecutionProgress(&result.Progress, item)
	}
	return result
}

func projectExecutionWorkItemView(
	snapshot *protocol.ExecutionSnapshot,
	view executionContextView,
	planItem protocol.ExecutionPlanItem,
	workItem protocol.WorkItem,
	spec protocol.WorkItemSpec,
) protocol.ExecutionWorkItemView {
	state := view.states[workItem.ID]
	dependencyIDs := make([]string, 0, len(view.dependencies[workItem.ID]))
	for _, dependency := range view.dependencies[workItem.ID] {
		dependencyIDs = append(dependencyIDs, dependency.DependsOnWorkItemID)
	}
	slices.Sort(dependencyIDs)

	item := protocol.ExecutionWorkItemView{
		ID:                 workItem.ID,
		LogicalKey:         workItem.LogicalKey,
		Kind:               workItem.Kind,
		Subject:            spec.Subject,
		Objective:          spec.Objective,
		Deliverable:        spec.Deliverable,
		AcceptanceCriteria: slices.Clone(spec.AcceptanceCriteria),
		InputRefs:          slices.Clone(spec.InputRefs),
		OutputScopes:       view.outputScopes(workItem.ID, spec.ID),
		DependencyIDs:      dependencyIDs,
		ParentWorkItemID:   planItem.ParentWorkItemID,
		Required:           planItem.Required,
		Terminal:           planItem.Terminal,
		Position:           planItem.Position,
		BlockReason:        state.BlockReason,
		NeededInput:        state.NeededInput,
		UpdatedAt:          state.UpdatedAt,
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = snapshot.Execution.UpdatedAt
	}
	if assignment := latestAssignmentForCurrentSpec(
		snapshot,
		workItem.ID,
		spec.ID,
	); assignment != nil {
		item.OwnerAgentID = assignment.OwnerAgentID
		item.AssignmentID = assignment.ID
		item.AssignmentStatus = assignment.Status
		item.AssignmentStrategy = assignment.Strategy
	}
	item.Attempts = projectExecutionAttempts(snapshot, workItem.ID, spec.ID)
	if submission, exists := view.submissions[workItem.ID]; exists &&
		submission.SpecID == spec.ID {
		item.Submission = &protocol.ExecutionSubmissionView{
			ID:               submission.ID,
			SubmitterAgentID: submission.SubmitterAgentID,
			ResultSummary:    submission.ResultSummary,
			ResultRefs:       slices.Clone(submission.ResultRefs),
			Evidence:         slices.Clone(submission.Evidence),
			CreatedAt:        submission.CreatedAt,
		}
		if acceptance, reviewed := view.acceptances[submission.ID]; reviewed {
			item.Acceptance = &protocol.ExecutionAcceptanceView{
				ID:              acceptance.ID,
				Decision:        acceptance.Decision,
				ReviewerKind:    acceptance.ReviewerKind,
				ReviewerID:      acceptance.ReviewerID,
				CriteriaResults: slices.Clone(acceptance.CriteriaResults),
				Feedback:        acceptance.Feedback,
				CreatedAt:       acceptance.CreatedAt,
			}
		}
	}
	item.Status = resolveExecutionWorkItemViewStatus(snapshot, view, planItem, item)
	return item
}

func projectExecutionAttempts(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) []protocol.ExecutionAttemptView {
	attempts := make([]protocol.ExecutionAttemptView, 0)
	for _, attempt := range snapshot.Attempts {
		if attempt.WorkItemID != workItemID || attempt.SpecID != specID {
			continue
		}
		attempts = append(attempts, protocol.ExecutionAttemptView{
			ID:              attempt.ID,
			AssignmentID:    attempt.AssignmentID,
			ParentAttemptID: attempt.ParentAttemptID,
			ExecutorKind:    attempt.ExecutorKind,
			ExecutorAgentID: attempt.ExecutorAgentID,
			ParentAgentID:   attempt.ParentAgentID,
			Status:          attempt.Status,
			FailureReason:   attempt.FailureReason,
			CreatedAt:       attempt.CreatedAt,
			StartedAt:       attempt.StartedAt,
			FinishedAt:      attempt.FinishedAt,
		})
	}
	slices.SortFunc(attempts, func(left, right protocol.ExecutionAttemptView) int {
		if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	return attempts
}

func resolveExecutionWorkItemViewStatus(
	snapshot *protocol.ExecutionSnapshot,
	view executionContextView,
	planItem protocol.ExecutionPlanItem,
	item protocol.ExecutionWorkItemView,
) protocol.ExecutionWorkItemViewStatus {
	if item.Acceptance != nil {
		switch item.Acceptance.Decision {
		case protocol.WorkAcceptanceAccepted:
			return protocol.ExecutionWorkItemViewAccepted
		case protocol.WorkAcceptanceRejected, protocol.WorkAcceptanceChangesRequested:
			return protocol.ExecutionWorkItemViewChangesRequested
		}
	}
	state := view.states[planItem.WorkItemID]
	if state.Status == protocol.WorkItemStatusCancelled ||
		state.Status == protocol.WorkItemStatusSuperseded {
		return protocol.ExecutionWorkItemViewCancelled
	}
	if item.Submission != nil {
		return protocol.ExecutionWorkItemViewSubmitted
	}
	if state.Status == protocol.WorkItemStatusWaitingInput {
		return protocol.ExecutionWorkItemViewBlocked
	}
	for index := len(item.Attempts) - 1; index >= 0; index-- {
		attempt := item.Attempts[index]
		if attempt.ParentAttemptID != "" {
			continue
		}
		switch attempt.Status {
		case protocol.WorkAttemptStatusRunning:
			return protocol.ExecutionWorkItemViewRunning
		case protocol.WorkAttemptStatusFailed,
			protocol.WorkAttemptStatusInterrupted,
			protocol.WorkAttemptStatusTimedOut:
			return protocol.ExecutionWorkItemViewFailed
		}
		break
	}
	if _, assigned := view.currentAssignments[planItem.WorkItemID]; assigned {
		return protocol.ExecutionWorkItemViewAssigned
	}
	if view.ready[planItem.WorkItemID] {
		return protocol.ExecutionWorkItemViewReady
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusCancelled ||
		snapshot.Execution.Status == protocol.ExecutionStatusSuperseded ||
		snapshot.Execution.Status == protocol.ExecutionStatusFailed {
		return protocol.ExecutionWorkItemViewCancelled
	}
	return protocol.ExecutionWorkItemViewWaiting
}

func incrementExecutionProgress(
	progress *protocol.ExecutionProgressView,
	item protocol.ExecutionWorkItemView,
) {
	progress.Total++
	if item.Required {
		progress.Required++
	}
	switch item.Status {
	case protocol.ExecutionWorkItemViewAccepted:
		progress.Accepted++
	case protocol.ExecutionWorkItemViewRunning, protocol.ExecutionWorkItemViewAssigned:
		progress.Running++
	case protocol.ExecutionWorkItemViewBlocked:
		progress.Blocked++
	case protocol.ExecutionWorkItemViewSubmitted:
		progress.Submitted++
	case protocol.ExecutionWorkItemViewReady:
		progress.Ready++
	case protocol.ExecutionWorkItemViewChangesRequested:
		progress.ChangesRequested++
	case protocol.ExecutionWorkItemViewFailed:
		progress.Failed++
	case protocol.ExecutionWorkItemViewCancelled:
		progress.Cancelled++
	default:
		progress.Waiting++
	}
}
