package tool

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalCompletionPayloadIncludesBudgetReportInstruction(t *testing.T) {
	budget := int64(100)
	payload := goalCompletionPayload(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{TotalTokens: 42},
		TimeUsedSeconds: 90,
	})

	report, ok := payload["completion_budget_report"].(string)
	if !ok || report == "" {
		t.Fatalf("completion_budget_report = %#v, want instruction", payload["completion_budget_report"])
	}
	for _, want := range []string{"goal.usage.total_tokens", "goal.token_budget", "goal.time_used_seconds"} {
		if !strings.Contains(report, want) {
			t.Fatalf("completion_budget_report missing %q: %s", want, report)
		}
	}
	budgetReport, ok := payload["budget_report"].(map[string]any)
	if !ok {
		t.Fatalf("budget_report = %#v, want map", payload["budget_report"])
	}
	if budgetReport["tokens_used"] != int64(42) || budgetReport["token_budget"] != int64(100) {
		t.Fatalf("budget_report = %#v, want token usage and budget", budgetReport)
	}
}

func TestGoalPayloadOmitsCompletionBudgetReportOutsideCompletion(t *testing.T) {
	budget := int64(100)
	payload := goalPayload(&protocol.Goal{
		Status:      protocol.GoalStatusActive,
		TokenBudget: &budget,
		Usage:       protocol.GoalUsage{TotalTokens: 42},
	})

	if _, ok := payload["completion_budget_report"]; ok {
		t.Fatalf("completion_budget_report present for non-completion payload: %#v", payload)
	}
}

func TestGoalCompletionPayloadOmitsReportWithoutUsageToReport(t *testing.T) {
	payload := goalCompletionPayload(&protocol.Goal{
		Status: protocol.GoalStatusComplete,
	})

	if _, ok := payload["completion_budget_report"]; ok {
		t.Fatalf("completion_budget_report present without budget or time: %#v", payload)
	}
}
