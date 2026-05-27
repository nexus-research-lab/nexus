package tool

import (
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalCompletionPayloadIncludesBudgetReportInstruction(t *testing.T) {
	budget := int64(100)
	payload := goalCompletionPayload(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Finish parity",
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{TotalTokens: 42},
		TimeUsedSeconds: 90,
		CreatedAt:       time.Unix(10, 0).UTC(),
		UpdatedAt:       time.Unix(20, 0).UTC(),
	})

	report, ok := payload["completionBudgetReport"].(string)
	if !ok || report == "" {
		t.Fatalf("completionBudgetReport = %#v, want instruction", payload["completionBudgetReport"])
	}
	for _, want := range []string{"goal.tokensUsed", "goal.tokenBudget", "goal.timeUsedSeconds"} {
		if !strings.Contains(report, want) {
			t.Fatalf("completionBudgetReport missing %q: %s", want, report)
		}
	}
	if payload["remainingTokens"] != int64(58) {
		t.Fatalf("remainingTokens = %#v, want 58", payload["remainingTokens"])
	}
	goal, ok := payload["goal"].(map[string]any)
	if !ok {
		t.Fatalf("goal = %#v, want map", payload["goal"])
	}
	wantGoal := map[string]any{
		"threadId":        "agent:nexus:ws:dm:chat",
		"objective":       "Finish parity",
		"status":          "complete",
		"tokenBudget":     int64(100),
		"tokensUsed":      int64(42),
		"timeUsedSeconds": int64(90),
		"createdAt":       int64(10),
		"updatedAt":       int64(20),
	}
	for key, want := range wantGoal {
		if goal[key] != want {
			t.Fatalf("goal[%s] = %#v, want %#v; goal=%#v", key, goal[key], want, goal)
		}
	}
}

func TestGoalPayloadOmitsCompletionBudgetReportOutsideCompletion(t *testing.T) {
	budget := int64(100)
	payload := goalPayload(&protocol.Goal{
		Status:      protocol.GoalStatusActive,
		TokenBudget: &budget,
		Usage:       protocol.GoalUsage{TotalTokens: 42},
	})

	if payload["completionBudgetReport"] != nil {
		t.Fatalf("completionBudgetReport = %#v, want nil", payload["completionBudgetReport"])
	}
}

func TestGoalCompletionPayloadOmitsReportWithoutUsageToReport(t *testing.T) {
	payload := goalCompletionPayload(&protocol.Goal{
		Status: protocol.GoalStatusComplete,
	})

	if payload["completionBudgetReport"] != nil {
		t.Fatalf("completionBudgetReport = %#v, want nil", payload["completionBudgetReport"])
	}
}

func TestGoalPayloadUsesCodexStatusNames(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		Status: protocol.GoalStatusBudgetLimited,
	})
	goal, ok := payload["goal"].(map[string]any)
	if !ok {
		t.Fatalf("goal = %#v, want map", payload["goal"])
	}
	if goal["status"] != "budgetLimited" {
		t.Fatalf("status = %#v, want budgetLimited", goal["status"])
	}
}

func TestGoalPayloadOmitsTokenBudgetWhenUnset(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		Status:     protocol.GoalStatusActive,
		SessionKey: "agent:nexus:ws:dm:chat",
	})
	goal, ok := payload["goal"].(map[string]any)
	if !ok {
		t.Fatalf("goal = %#v, want map", payload["goal"])
	}
	if _, exists := goal["tokenBudget"]; exists {
		t.Fatalf("goal = %#v, want omitted tokenBudget", goal)
	}
}
