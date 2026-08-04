// INPUT: 当前 transient Execution 的授权、完成条件与结构化持续性证据。
// OUTPUT: hard availability blockers 与可选 persistence 建议信号。
// POS: 后端只裁决权限/状态；是否值得提升由 Agent 决定。
package orchestration

import "strings"

// GoalPromotionSignal 是帮助 Agent 判断 persistence 的权威建议事实。
type GoalPromotionSignal string

const (
	GoalPromotionSignalObservedBoundary GoalPromotionSignal = "observed_boundary"
	GoalPromotionSignalRoomDependency   GoalPromotionSignal = "room_dependency"
	GoalPromotionSignalExternalWait     GoalPromotionSignal = "external_wait"
	GoalPromotionSignalScheduledRetry   GoalPromotionSignal = "scheduled_retry"
	GoalPromotionSignalRecovery         GoalPromotionSignal = "recovery"
	GoalPromotionSignalContextBoundary  GoalPromotionSignal = "context_boundary"
)

// AdaptiveGoalEvidence 是后端能够验证的 promotion 输入。
//
// Complexity、Plan 长度、Room 成员数量和 subagent 数量故意不在此结构中：
// 它们属于 Agent 的语义判断，不需要伪装成后端可证明的授权事实。
type AdaptiveGoalEvidence struct {
	ExecutionID                 string
	ObjectiveClear              bool
	CompletionCriteriaAvailable bool
	ScopeAuthorizesContinuation bool
	PlanMode                    bool
	PromotionPolicyUnavailable  bool
	AutomaticGoalDisabled       bool
	ExistingGoalID              string
	ConflictingGoalID           string
	RequiredWorkRemaining       bool
	NewAuthorityRequired        bool

	ObservedBoundaryWithRequiredWork bool
	BoundRoomDependency              bool
	RequiredExternalWait             bool
	ScheduledRetry                   bool
	RecoveryRequired                 bool
	PredictedContextBoundary         bool
}

// AdaptiveGoalDecision 解释 promotion 是否被允许，以及参与判定的证据。
type AdaptiveGoalDecision struct {
	Promote  bool
	Signals  []GoalPromotionSignal
	Blockers []string
}

// EvaluateAdaptiveGoalPromotion 分离硬 availability 与建议 signal。
// 缺少 signal 不再阻止 Agent 基于任务语义选择 Goal。
func EvaluateAdaptiveGoalPromotion(evidence AdaptiveGoalEvidence) AdaptiveGoalDecision {
	decision := AdaptiveGoalDecision{}
	if strings.TrimSpace(evidence.ExecutionID) == "" {
		decision.Blockers = append(decision.Blockers, "execution_missing")
	}
	if !evidence.ObjectiveClear {
		decision.Blockers = append(decision.Blockers, "objective_unclear")
	}
	if !evidence.CompletionCriteriaAvailable {
		decision.Blockers = append(decision.Blockers, "completion_criteria_missing")
	}
	if !evidence.ScopeAuthorizesContinuation {
		decision.Blockers = append(decision.Blockers, "continuation_not_authorized")
	}
	if evidence.NewAuthorityRequired {
		decision.Blockers = append(decision.Blockers, "new_authority_required")
	}
	if evidence.PlanMode {
		decision.Blockers = append(decision.Blockers, "plan_mode")
	}
	if evidence.PromotionPolicyUnavailable {
		decision.Blockers = append(decision.Blockers, "goal_policy_unavailable")
	}
	if evidence.AutomaticGoalDisabled {
		decision.Blockers = append(decision.Blockers, "automatic_goal_disabled")
	}
	if strings.TrimSpace(evidence.ExistingGoalID) != "" {
		decision.Blockers = append(decision.Blockers, "goal_already_active")
	}
	if strings.TrimSpace(evidence.ConflictingGoalID) != "" {
		decision.Blockers = append(decision.Blockers, "goal_conflict")
	}
	if len(decision.Blockers) > 0 {
		return decision
	}

	if evidence.ObservedBoundaryWithRequiredWork {
		decision.Signals = append(decision.Signals, GoalPromotionSignalObservedBoundary)
	}
	if evidence.BoundRoomDependency {
		decision.Signals = append(decision.Signals, GoalPromotionSignalRoomDependency)
	}
	if evidence.RequiredExternalWait {
		decision.Signals = append(decision.Signals, GoalPromotionSignalExternalWait)
	}
	if evidence.ScheduledRetry {
		decision.Signals = append(decision.Signals, GoalPromotionSignalScheduledRetry)
	}
	if evidence.RecoveryRequired {
		decision.Signals = append(decision.Signals, GoalPromotionSignalRecovery)
	}
	if evidence.PredictedContextBoundary {
		decision.Signals = append(decision.Signals, GoalPromotionSignalContextBoundary)
	}
	decision.Promote = true
	return decision
}
