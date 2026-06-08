package tool

import (
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalCompletionPayloadIncludesFinalUsageReport(t *testing.T) {
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
	if !strings.Contains(report, "最终 Goal 用量：42 / 100 tokens，剩余 58 tokens，耗时约 1m30s。") {
		t.Fatalf("completionBudgetReport = %q, want final usage line", report)
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

func TestStructuredResultTextUsesCodexFieldOrder(t *testing.T) {
	budget := int64(100)
	result := structuredResult("goal marked complete", goalCompletionPayload(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Finish parity",
		TokenBudget:     &budget,
		Usage:           protocol.GoalUsage{TotalTokens: 42},
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
  "completionBudgetReport": "Goal achieved. Send one concise final response now, then stop and wait for user input. Do not call more tools or start new work. State that this tracked Goal is complete and ready to be cleared; do not describe it as paused. Briefly summarize what ` + "`goal.objective`" + ` achieved. Include this exact final usage line in the response: ` + "`最终 Goal 用量：42 / 100 tokens，剩余 58 tokens，耗时约 1m30s。`" + `"
}`
	if text != want {
		t.Fatalf("text content = %s, want %s", text, want)
	}
}

func TestGoalCompletionReportFormatsCodexStyleUsageLine(t *testing.T) {
	report := completionBudgetReport(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		Usage:           protocol.GoalUsage{TotalTokens: 603673},
		TimeUsedSeconds: 23*60 + 4,
	})

	if !strings.Contains(report, "最终 Goal 用量：603,673 tokens，耗时约 23m4s。") {
		t.Fatalf("completionBudgetReport = %q, want Codex-style usage line", report)
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
	if !strings.Contains(report, "最终 Goal 用量：0 tokens，耗时约 0s。") {
		t.Fatalf("completionBudgetReport = %q, want zero usage line", report)
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
