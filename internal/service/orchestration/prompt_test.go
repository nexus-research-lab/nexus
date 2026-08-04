package orchestration

import (
	"strings"
	"testing"
)

func TestStablePromptDefinesExecutionBoundaries(t *testing.T) {
	prompt := StablePrompt()
	for _, expected := range []string{
		"Deliver the task itself first",
		"These primitives are optional choices, not a mandatory pipeline",
		"Task/Todo is a local checklist inside the current Agent node",
		"Casual Room chat",
		"without creating a Plan",
		"Choose each primitive independently from task facts",
		"minimum structure whose value exceeds its coordination cost",
		"load the `execution-orchestrator` Skill",
		"Its guidance is advisory",
		"native `items` object array",
		"reviewer may be the owner, Lead, or another authorized Agent",
		"Self-review is valid",
		"Graph UI already display",
		"`allowed_actions`",
		"Use Room `@` messages",
		"with or without a following space",
		"records Tool and Subagent Node Runs automatically",
		"Do not stop just to ask the user to send “continue”",
		"adaptively promoted",
		"Loop does not require the full Goal lifecycle",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("stable execution prompt missing %q", expected)
		}
	}
}
