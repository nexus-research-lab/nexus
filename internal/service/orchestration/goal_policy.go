// INPUT: 当前 transient Execution 的授权、完成条件与结构化持续性证据。
// OUTPUT: 可审计、fail-closed 的 adaptive Goal promotion 决策。
// POS: 模型可提出 promotion，但后端是否允许自动持久化的唯一纯规则入口。
package orchestration

import "strings"

// GoalPromotionSignal 是允许 transient Execution 自适应提升的持久性证据。
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
// 它们可以帮助模型建图，但不能成为自动持久化授权。
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

// EvaluateAdaptiveGoalPromotion 对自动 Goal 提升执行硬门槛 + durable signal 判定。
//
// 未知或缺失事实按不允许处理；这条规则不处理用户显式 create_goal。
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
	if !evidence.RequiredWorkRemaining {
		decision.Blockers = append(decision.Blockers, "no_required_work_remaining")
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
	if len(decision.Signals) == 0 {
		decision.Blockers = append(decision.Blockers, "durable_signal_missing")
		return decision
	}
	decision.Promote = true
	return decision
}
