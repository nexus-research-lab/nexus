package operation

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
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
	const wantReport = "Goal achieved. Use the next final response as the complete user-facing delivery. It must stand on its own and satisfy `goal.objective`: include the full requested content when content itself is the deliverable; for files or artifacts, provide exact links or paths; for implementation, research, or external-state work, present the key outcomes and relevant verification. Do not make Goal completion the headline or replace the result with a completion notice or brief summary; mention completion only secondarily if useful. Then stop and wait for user input."
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
    "objectiveRevision": 1,
    "completionCriteria": [],
    "status": "complete",
    "tokenBudget": 100,
    "tokensUsed": 42,
    "timeUsedSeconds": 90,
    "createdAt": 10,
    "updatedAt": 20
  },
  "remainingTokens": 58,
  "completionBudgetReport": "Goal achieved. Use the next final response as the complete user-facing delivery. It must stand on its own and satisfy ` + "`goal.objective`" + `: include the full requested content when content itself is the deliverable; for files or artifacts, provide exact links or paths; for implementation, research, or external-state work, present the key outcomes and relevant verification. Do not make Goal completion the headline or replace the result with a completion notice or brief summary; mention completion only secondarily if useful. Then stop and wait for user input."
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

func TestGoalCompletionReportPrioritizesResultDeliveryWithoutUsageDetails(t *testing.T) {
	report := completionBudgetReport(&protocol.Goal{
		Status:          protocol.GoalStatusComplete,
		Usage:           protocol.GoalUsage{TotalTokens: 42, ActualTotalTokens: 603673},
		TimeUsedSeconds: 23*60 + 4,
	})

	for _, expected := range []string{
		"complete user-facing delivery",
		"stand on its own",
		"include the full requested content",
		"provide exact links or paths",
		"key outcomes and relevant verification",
		"Do not make Goal completion the headline",
		"mention completion only secondarily",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("completionBudgetReport = %q, want result-first guidance %q", report, expected)
		}
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
	for _, completionOnly := range []string{"goalId", "usageFinalized", "completionUsageCheckpointReport"} {
		if value, exists := payload[completionOnly]; exists {
			t.Fatalf("payload[%q] = %#v, want field omitted outside completion receipt", completionOnly, value)
		}
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

func TestGoalPayloadIncludesAuthoritativeObjectiveBoundary(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		Status:     protocol.GoalStatusActive,
		SessionKey: "room:group:conversation-1",
		Objective:  "Ship the verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(3),
			protocol.GoalMetadataCompletionCriteria: []any{
				" report exists ",
				"all sections are verified",
			},
		},
	})
	goal, ok := payload["goal"].(map[string]any)
	if !ok || goal["objectiveRevision"] != int64(3) {
		t.Fatalf("goal = %#v", payload["goal"])
	}
	criteria, ok := goal["completionCriteria"].([]string)
	if !ok || !slices.Equal(criteria, []string{
		"report exists",
		"all sections are verified",
	}) {
		t.Fatalf("completion criteria = %#v", goal["completionCriteria"])
	}
	textResult := structuredResult("current goal loaded", payload)
	text, _ := textResult.Content[0]["text"].(string)
	if !strings.Contains(text, `"objectiveRevision": 3`) ||
		!strings.Contains(text, `"completionCriteria":`) ||
		!strings.Contains(text, `"all sections are verified"`) {
		t.Fatalf("text payload omitted objective boundary: %s", text)
	}
}

func TestGoalPayloadDirectsPendingObjectiveTransitionToSuccessorPlan(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		ID:         "goal-rebase",
		Status:     protocol.GoalStatusActive,
		SessionKey: "agent:nexus:ws:dm:goal-rebase",
		Objective:  "Revised objective",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(2),
			protocol.GoalMetadataExecutionID:       "execution-successor",
			protocol.GoalMetadataObjectiveTransition: map[string]any{
				"transition_id":          "transition-1",
				"command_id":             "command-1",
				"phase":                  "awaiting_plan",
				"old_revision":           int64(1),
				"new_revision":           int64(2),
				"old_execution_id":       "execution-old",
				"successor_execution_id": "execution-successor",
				"target_objective":       "Revised objective",
				"reason":                 "user changed scope",
				"source":                 "model",
			},
		},
	})
	action, ok := payload["nextAction"].(map[string]any)
	if !ok || action["domain"] != command.DomainExecution || action["operation"] != "prepare_plan_execution" ||
		!strings.Contains(action["reason"].(string), "successor WorkGraph") {
		t.Fatalf("nextAction = %#v", payload["nextAction"])
	}
	textResult := structuredResult("goal retargeted", payload)
	text, _ := textResult.Content[0]["text"].(string)
	if !strings.Contains(text, `"nextAction"`) ||
		!strings.Contains(text, `"prepare_plan_execution"`) {
		t.Fatalf("structured result omitted next action: %s", text)
	}
}

func TestGoalPayloadDirectsPreparedObjectiveTransitionBackToRetarget(t *testing.T) {
	payload := goalPayload(&protocol.Goal{
		ID:         "goal-rebase",
		Status:     protocol.GoalStatusActive,
		SessionKey: "agent:nexus:ws:dm:goal-rebase",
		Objective:  "Original objective",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveTransition: map[string]any{
				"transition_id":          "transition-1",
				"command_id":             "command-1",
				"phase":                  "prepared",
				"old_revision":           int64(1),
				"new_revision":           int64(2),
				"old_execution_id":       "execution-old",
				"successor_execution_id": "execution-successor",
				"target_objective":       "Revised objective",
				"reason":                 "user changed scope",
				"source":                 "model",
			},
		},
	})
	action, ok := payload["nextAction"].(map[string]any)
	if !ok || action["domain"] != command.DomainGoal || action["operation"] != "retarget_goal" ||
		action["targetObjective"] != "Revised objective" {
		t.Fatalf("nextAction = %#v", payload["nextAction"])
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
    "objectiveRevision": 1,
    "completionCriteria": [],
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
