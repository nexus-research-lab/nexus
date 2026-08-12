// INPUT: trusted Room coordinator identity、physical round identity 与 explicit get/materialized Plan、exact Goal continuation 或 resolved ReviewBinding transition。
// OUTPUT: round-scoped Coordination capability 的 mint、review-to-coordination 升级、检查与释放。
// POS: conversation substrate 到 Execution coordination overlay 的后端准入边界。
package orchestration

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ActivateRuntimeCoordination 在显式 get 或受信 mutation/continuation 边界把当前
// 物理 round 升级为协调 capability。隐式 round-start context、聊天正文或 coordinator 名称都不调用它。
func (s *Service) ActivateRuntimeCoordination(
	_ context.Context,
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) error {
	if snapshot == nil {
		return nil
	}
	if err := requireCoordinator(actor, snapshot); err != nil {
		return err
	}
	if !roomConversationCoordinator(actor, snapshot) &&
		!exactGoalCoordinator(actor, snapshot) {
		return nil
	}
	return s.mintRuntimeCoordination(actor, snapshot.Execution.ID)
}

func (s *Service) mintRuntimeCoordination(
	actor ActorContext,
	executionID string,
) error {
	key := runtimeCoordinationRoundKey(actor)
	if key == "" {
		return domainError(
			ErrorCodeConversationOnly,
			"Room coordination transition requires an exact runtime round identity",
		)
	}
	s.coordinationMu.Lock()
	if s.coordinationRounds == nil {
		s.coordinationRounds = make(map[string]string)
	}
	s.coordinationRounds[key] = strings.TrimSpace(executionID)
	s.coordinationMu.Unlock()
	return nil
}

// ReleaseRuntimeCoordination 清除物理 round 的临时协调 capability。
func (s *Service) ReleaseRuntimeCoordination(actor ActorContext) {
	if s == nil {
		return
	}
	key := runtimeCoordinationRoundKey(actor)
	if key == "" {
		return
	}
	s.coordinationMu.Lock()
	delete(s.coordinationRounds, key)
	s.coordinationMu.Unlock()
}

func (s *Service) activateRuntimeCoordinationResult(
	ctx context.Context,
	actor ActorContext,
	result MutationResult,
) MutationResult {
	if actor.PlanMode || result.Snapshot == nil {
		return result
	}
	if result.Outcome != MutationApplied && result.Outcome != MutationNoOp {
		return result
	}
	_ = s.ActivateRuntimeCoordination(ctx, actor, result.Snapshot)
	return result
}

// activateReviewContinuationResult 在 coordinator 通过 trusted ReviewBinding
// 或 selected self-review WorkBinding 提交唯一 Acceptance 后，把同一物理
// round 升回 coordination。用户不需要另发一条“继续”。
func (s *Service) activateReviewContinuationResult(
	actor ActorContext,
	result MutationResult,
) MutationResult {
	if actor.PlanMode ||
		result.Snapshot == nil ||
		(result.Outcome != MutationApplied && result.Outcome != MutationNoOp) ||
		!isCurrentExecutionStatus(result.Snapshot.Execution.Status) {
		return result
	}
	if !reviewBindingResolved(actor, result.Snapshot) &&
		!workBindingReviewResolved(actor, result.Snapshot) {
		return result
	}
	if err := requireCoordinator(actor, result.Snapshot); err != nil {
		return result
	}
	if err := s.mintRuntimeCoordination(actor, result.Snapshot.Execution.ID); err != nil {
		return result
	}
	result.NextActions = nextActions(
		result.Snapshot,
		s.effectiveRuntimeCoordinationActor(actor, result.Snapshot),
	)
	return result
}

// effectiveRuntimeCoordinationActor 消费已完成的 ReviewBinding 展示权限，
// 但只在同一 round 已由后端 mint coordination capability 后清除其 review lane。
func (s *Service) effectiveRuntimeCoordinationActor(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) ActorContext {
	if snapshot != nil && s.runtimeCoordinationActive(actor, snapshot.Execution.ID) {
		if actor.ReviewBinding != nil && reviewBindingResolved(actor, snapshot) {
			actor.ReviewBinding = nil
		}
		if actor.WorkBinding != nil && workBindingReviewResolved(actor, snapshot) {
			actor.WorkBinding = nil
		}
	}
	return actor
}

func workBindingReviewResolved(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) bool {
	if snapshot == nil || actor.WorkBinding == nil {
		return false
	}
	binding := normalizeExecutionWorkBinding(actor.WorkBinding)
	assignment := findAssignmentByID(snapshot, binding.AssignmentID)
	if assignment == nil ||
		assignment.ExecutionID != binding.ExecutionID ||
		assignment.PlanID != binding.PlanID ||
		assignment.WorkItemID != binding.WorkItemID ||
		assignment.SpecID != binding.SpecID ||
		strings.TrimSpace(assignment.ReturnToAgentID) != strings.TrimSpace(actor.AgentID) {
		return false
	}
	for index := range snapshot.Submissions {
		submission := &snapshot.Submissions[index]
		if submission.AssignmentID == assignment.ID &&
			submission.WorkItemID == assignment.WorkItemID &&
			submission.SpecID == assignment.SpecID &&
			acceptanceForSubmission(snapshot, submission.ID) != nil {
			return true
		}
	}
	return false
}

func reviewBindingResolved(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) bool {
	if snapshot == nil || actor.ReviewBinding == nil {
		return false
	}
	binding := normalizeExecutionReviewBinding(actor.ReviewBinding)
	if !completeExecutionReviewBinding(binding) ||
		binding.ExecutionID != strings.TrimSpace(snapshot.Execution.ID) ||
		binding.TargetAgentID != strings.TrimSpace(actor.AgentID) {
		return false
	}
	for _, acceptance := range snapshot.Acceptances {
		if acceptance.SubmissionID == binding.SubmissionID &&
			acceptance.ExecutionID == binding.ExecutionID &&
			acceptance.PlanID == binding.PlanID &&
			acceptance.WorkItemID == binding.WorkItemID &&
			acceptance.SpecID == binding.SpecID &&
			acceptance.AssignmentID == binding.AssignmentID {
			return true
		}
	}
	return false
}

func (s *Service) requireRuntimeCoordination(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) error {
	if !roomConversationCoordinator(actor, snapshot) {
		return nil
	}
	if s.runtimeCoordinationActive(actor, snapshot.Execution.ID) {
		return nil
	}
	return domainError(
		ErrorCodeConversationOnly,
		"this Room round is conversational; call get_execution to inspect and enter current coordination, or prepare_plan_execution then plan_execution to materialize a revised Plan before other Execution mutations",
	)
}

func (s *Service) runtimeCoordinationActive(
	actor ActorContext,
	executionID string,
) bool {
	if s == nil {
		return false
	}
	key := runtimeCoordinationRoundKey(actor)
	if key == "" {
		return false
	}
	s.coordinationMu.RLock()
	boundExecutionID := strings.TrimSpace(s.coordinationRounds[key])
	s.coordinationMu.RUnlock()
	return boundExecutionID != "" &&
		boundExecutionID == strings.TrimSpace(executionID)
}

func roomConversationCoordinator(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) bool {
	return unboundRoomConversationActor(actor, snapshot) &&
		strings.TrimSpace(actor.AgentID) ==
			strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
}

func exactGoalCoordinator(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
) bool {
	if snapshot == nil {
		return false
	}
	return actor.ScopeKind == protocol.ExecutionScopeRoom &&
		normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorAgent &&
		strings.TrimSpace(actor.GoalID) != "" &&
		actor.GoalObjectiveRevision > 0 &&
		strings.TrimSpace(actor.GoalID) ==
			strings.TrimSpace(snapshot.Execution.GoalID) &&
		actor.GoalObjectiveRevision == snapshot.Execution.GoalObjectiveRevision &&
		strings.TrimSpace(actor.AgentID) ==
			strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
}

func runtimeCoordinationRoundKey(actor ActorContext) string {
	if actor.ScopeKind != protocol.ExecutionScopeRoom ||
		normalizeActorKind(actor.ActorKind) != protocol.ExecutionActorAgent {
		return ""
	}
	roundID := firstCoordinationValue(
		actor.RuntimeRoundID,
		actor.AgentRoundID,
		actor.RootRoundID,
	)
	if roundID == "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(actor.OwnerUserID),
		strings.TrimSpace(actor.SessionKey),
		strings.TrimSpace(actor.AgentID),
		roundID,
	}, "\x00")
}

func firstCoordinationValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
