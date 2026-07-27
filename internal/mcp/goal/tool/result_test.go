package tool

import (
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalCompletionPayloadIncludesUsageCheckpointReport(t *testing.T) {
	budget := int64(100)
	payload := goalCompletionPayload(&protocol.Goal{
		ID:              "goal-1",
		Status:          protocol.GoalStatusComplete,
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Finish parity",
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{TotalTokens: 42, ActualTotalTokens: 130},
		TimeUsedSeconds: 90,
		CreatedAt:       time.Unix(10, 0).UTC(),
		UpdatedAt:       time.Unix(20, 0).UTC(),
	})

	report, ok := payload["completionBudgetReport"].(string)
	if !ok || report == "" {
		t.Fatalf("completionBudgetReport = %#v, want instruction", payload["completionBudgetReport"])
	}
	const wantReport = "Goal achieved. Send one concise final response stating that the Goal is complete and briefly summarizing what `goal.objective` achieved. Then stop and wait for user input."
	if report != wantReport {
		t.Fatalf("completionBudgetReport = %q, want %q", report, wantReport)
	}
	if payload["completionUsageCheckpointReport"] != report ||
		payload["goalId"] != "goal-1" ||
		payload["usageFinalized"] != false {
		t.Fatalf("completion checkpoint metadata = %#v", payload)
	}
	if payload["remainingTokens"] != int64(58) {
		t.Fatalf("remainingTokens = %#v, want 58", payload["remainingTokens"])
	}
	goal, ok := payload["goal"].(map[string]any)
	if !ok {
		t.Fatalf("goal = %#v, want map", payload["goal"])
	}
	wantGoal := map[string]any{
		"threadId":              "agent:nexus:ws:dm:chat",
		"objective":             "Finish parity",
		"status":                "complete",
		"tokenBudget":           int64(100),
		"tokensUsed":            int64(42),
		"budgetTokens":          int64(42),
		"actualTokens":          int64(130),
		"actualTokensEstimated": false,
		"timeUsedSeconds":       int64(90),
		"createdAt":             int64(10),
		"updatedAt":             int64(20),
	}
	for key, want := range wantGoal {
		if goal[key] != want {
			t.Fatalf("goal[%s] = %#v, want %#v; goal=%#v", key, goal[key], want, goal)
		}
	}
}

func TestStructuredResultTextUsesCodexFieldOrder(t *testing.T) {
	budget := int64(100)
	result := structuredResult("goal marked complete", goalCompletionPayload(&protocol.Goal{
		ID:              "goal-1",
		Status:          protocol.GoalStatusComplete,
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Finish parity",
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{TotalTokens: 42, ActualTotalTokens: 130},
		TimeUsedSeconds: 90,
		CreatedAt:       time.Unix(10, 0).UTC(),
		UpdatedAt:       time.Unix(20, 0).UTC(),
	}))

	text, ok := result.Content[0]["text"].(string)
	if !ok {
		t.Fatalf("text content = %#v, want string", result.Content)
	}
	want := `{
  "goal": {
    "threadId": "agent:nexus:ws:dm:chat",
    "objective": "Finish parity",
    "status": "complete",
    "tokenBudget": 100,
    "tokensUsed": 42,
    "timeUsedSeconds": 90,
    "createdAt": 10,
    "updatedAt": 20
  },
  "remainingTokens": 58,
  "completionBudgetReport": "Goal achieved. Send one concise final response stating that the Goal is complete and briefly summarizing what ` + "`goal.objective`" + ` achieved. Then stop and wait for user input."
}`
	if text != want {
		t.Fatalf("text content = %s, want %s", text, want)
	}
	for _, hidden := range []string{"goalId", "usageFinalized", "completionUsageCheckpointReport", "budgetTokens", "actualTokens", "actualTokensEstimated"} {
		if strings.Contains(text, `"`+hidden+`"`) {
			t.Fatalf("text content exposes structured-only field %q: %s", hidden, text)
		}
	}
}

func TestGoalCompletionReportDoesNotRequireUsageDetails(t *testing.T) {
	report := completionBudgetReport(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		Usage:           protocol.GoalUsage{TotalTokens: 42, ActualTotalTokens: 603673},
		TimeUsedSeconds: 23*60 + 4,
	})

	if !strings.Contains(report, "Goal achieved") || !strings.Contains(report, "briefly summarizing") {
		t.Fatalf("completionBudgetReport = %q, want concise completion guidance", report)
	}
	for _, unwanted := range []string{"tokens", "elapsed", "耗时", "最终回复自身用量"} {
		if strings.Contains(report, unwanted) {
			t.Fatalf("completionBudgetReport = %q, should not expose %q", report, unwanted)
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

func TestGoalCompletionPayloadIncludesStopInstructionWithoutUsageToReport(t *testing.T) {
	payload := goalCompletionPayload(&protocol.Goal{
		Status: protocol.GoalStatusComplete,
	})

	report, ok := payload["completionBudgetReport"].(string)
	if !ok || !strings.Contains(report, "stop and wait for user input") {
		t.Fatalf("completionBudgetReport = %#v, want stop instruction", payload["completionBudgetReport"])
	}
	if strings.Contains(report, "tokens") || strings.Contains(report, "0s") {
		t.Fatalf("completionBudgetReport = %q, should not expose zero usage", report)
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

func TestGoalPayloadIncludesNullTokenBudgetWhenUnset(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		Status:     protocol.GoalStatusActive,
		SessionKey: "agent:nexus:ws:dm:chat",
	})
	goal, ok := payload["goal"].(map[string]any)
	if !ok {
		t.Fatalf("goal = %#v, want map", payload["goal"])
	}
	value, exists := goal["tokenBudget"]
	if !exists || value != nil {
		t.Fatalf("goal = %#v, want null tokenBudget", goal)
	}
}

func TestStructuredResultTextIncludesNullTokenBudget(t *testing.T) {
	result := structuredResult("current goal loaded", goalPayload(&protocol.Goal{
		Status:     protocol.GoalStatusActive,
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Unbudgeted work",
		CreatedAt:  time.Unix(10, 0).UTC(),
		UpdatedAt:  time.Unix(20, 0).UTC(),
	}))

	text, ok := result.Content[0]["text"].(string)
	if !ok {
		t.Fatalf("text content = %#v, want string", result.Content)
	}
	want := `{
  "goal": {
    "threadId": "agent:nexus:ws:dm:chat",
    "objective": "Unbudgeted work",
    "status": "active",
    "tokenBudget": null,
    "tokensUsed": 0,
    "timeUsedSeconds": 0,
    "createdAt": 10,
    "updatedAt": 20
  },
  "remainingTokens": null,
  "completionBudgetReport": null
}`
	if text != want {
		t.Fatalf("text content = %s, want %s", text, want)
	}
}
