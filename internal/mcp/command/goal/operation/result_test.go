package operation

import (
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
