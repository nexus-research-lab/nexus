// INPUT: trusted Room coordinator identity、physical round identity 与 explicit get/plan transition。
// OUTPUT: round-scoped Coordination capability 的 mint、检查与释放。
// POS: conversation substrate 到 Execution coordination overlay 的后端准入边界。
package orchestration

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ActivateRuntimeCoordination 把显式 get_execution 读取升级为当前物理 round
// 的协调 capability。普通 round-start context、聊天正文或 coordinator 名称都不调用它。
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
	s.coordinationRounds[key] = strings.TrimSpace(snapshot.Execution.ID)
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
		"this Room round is conversational; call get_execution to inspect current responsibility or plan_execution to enter coordination before other Execution mutations",
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
