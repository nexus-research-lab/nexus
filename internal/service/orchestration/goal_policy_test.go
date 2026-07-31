package orchestration

import (
	"slices"
	"testing"
)

func TestEvaluateAdaptiveGoalPromotionRequiresHardGatesAndDurableSignal(t *testing.T) {
	base := AdaptiveGoalEvidence{
		ExecutionID:                 "execution-1",
		ObjectiveClear:              true,
		CompletionCriteriaAvailable: true,
		ScopeAuthorizesContinuation: true,
		RequiredWorkRemaining:       true,
		BoundRoomDependency:         true,
		PredictedContextBoundary:    true,
	}
	decision := EvaluateAdaptiveGoalPromotion(base)
	if !decision.Promote {
		t.Fatalf("durable Room dependency should allow adaptive promotion: %+v", decision)
	}
	for _, want := range []GoalPromotionSignal{
		GoalPromotionSignalRoomDependency,
		GoalPromotionSignalContextBoundary,
	} {
		if !slices.Contains(decision.Signals, want) {
			t.Fatalf("promotion signal missing %q: %+v", want, decision)
		}
	}
}

func TestEvaluateAdaptiveGoalPromotionRejectsComplexityWithoutDurableEvidence(t *testing.T) {
	decision := EvaluateAdaptiveGoalPromotion(AdaptiveGoalEvidence{
		ExecutionID:                 "execution-1",
		ObjectiveClear:              true,
		CompletionCriteriaAvailable: true,
		ScopeAuthorizesContinuation: true,
		RequiredWorkRemaining:       true,
	})
	if decision.Promote || !slices.Contains(decision.Blockers, "durable_signal_missing") {
		t.Fatalf("complexity without a durable signal must fail closed: %+v", decision)
	}
}

func TestEvaluateAdaptiveGoalPromotionRejectsEveryHardBoundary(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdaptiveGoalEvidence)
		blocker string
	}{
		{name: "missing execution", mutate: func(value *AdaptiveGoalEvidence) {
			value.ExecutionID = ""
		}, blocker: "execution_missing"},
		{name: "unclear objective", mutate: func(value *AdaptiveGoalEvidence) {
			value.ObjectiveClear = false
		}, blocker: "objective_unclear"},
		{name: "missing criteria", mutate: func(value *AdaptiveGoalEvidence) {
			value.CompletionCriteriaAvailable = false
		}, blocker: "completion_criteria_missing"},
		{name: "scope", mutate: func(value *AdaptiveGoalEvidence) {
			value.ScopeAuthorizesContinuation = false
		}, blocker: "continuation_not_authorized"},
		{name: "new authority", mutate: func(value *AdaptiveGoalEvidence) {
			value.NewAuthorityRequired = true
		}, blocker: "new_authority_required"},
		{name: "plan mode", mutate: func(value *AdaptiveGoalEvidence) {
			value.PlanMode = true
		}, blocker: "plan_mode"},
		{name: "policy unavailable", mutate: func(value *AdaptiveGoalEvidence) {
			value.PromotionPolicyUnavailable = true
		}, blocker: "goal_policy_unavailable"},
		{name: "disabled", mutate: func(value *AdaptiveGoalEvidence) {
			value.AutomaticGoalDisabled = true
		}, blocker: "automatic_goal_disabled"},
		{name: "existing goal", mutate: func(value *AdaptiveGoalEvidence) {
			value.ExistingGoalID = "goal-1"
		}, blocker: "goal_already_active"},
		{name: "conflicting goal", mutate: func(value *AdaptiveGoalEvidence) {
			value.ConflictingGoalID = "goal-other"
		}, blocker: "goal_conflict"},
		{name: "completed", mutate: func(value *AdaptiveGoalEvidence) {
			value.RequiredWorkRemaining = false
		}, blocker: "no_required_work_remaining"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := AdaptiveGoalEvidence{
				ExecutionID:                      "execution-1",
				ObjectiveClear:                   true,
				CompletionCriteriaAvailable:      true,
				ScopeAuthorizesContinuation:      true,
				RequiredWorkRemaining:            true,
				ObservedBoundaryWithRequiredWork: true,
			}
			test.mutate(&evidence)
			decision := EvaluateAdaptiveGoalPromotion(evidence)
			if decision.Promote || !slices.Contains(decision.Blockers, test.blocker) {
				t.Fatalf("hard gate %q must reject promotion: %+v", test.blocker, decision)
			}
		})
	}
}

func TestEvaluateAdaptiveGoalPromotionReportsAllFailedHardGates(t *testing.T) {
	decision := EvaluateAdaptiveGoalPromotion(AdaptiveGoalEvidence{
		PlanMode:              true,
		AutomaticGoalDisabled: true,
		NewAuthorityRequired:  true,
	})
	for _, blocker := range []string{
		"execution_missing",
		"objective_unclear",
		"completion_criteria_missing",
		"continuation_not_authorized",
		"new_authority_required",
		"plan_mode",
		"automatic_goal_disabled",
		"no_required_work_remaining",
	} {
		if !slices.Contains(decision.Blockers, blocker) {
			t.Fatalf("decision should expose blocker %q: %+v", blocker, decision)
		}
	}
}
