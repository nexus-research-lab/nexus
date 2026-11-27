package operation

import (
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
