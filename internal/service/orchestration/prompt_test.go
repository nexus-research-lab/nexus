package orchestration

import (
	"strings"
	"testing"
)

func TestStablePromptDefinesExecutionBoundaries(t *testing.T) {
	prompt := StablePrompt()
	for _, expected := range []string{
		"Goal is a persistence boundary",
		"Complexity, a long Plan, Room membership, or subagent use alone never activates a Goal",
		"A Plan revision is immutable",
		"A worker result is a Submission, not Acceptance",
		"exact `allowed_actions`",
		"After assigning a produce Work Item, do not perform the same deliverable",
		"Use structured `assign_work` as the normal delegation path",
		"One or many raw mentions may address Room members for casual conversation",
		"participant count alone never requires a Plan",
		"A raw mention never activates an Assignment",
		"structured dispatch transports responsibility",
		"tracked deliverables that unlock dependencies",
		"capped at 32 items",
		"`projection_limit_exceeded`",
		"terminal integration or verification Work Item are Accepted",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("stable execution prompt missing %q", expected)
		}
	}
}
