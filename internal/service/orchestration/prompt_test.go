package orchestration

import (
	"strings"
	"testing"
)

func TestStablePromptDefinesExecutionBoundaries(t *testing.T) {
	prompt := StablePrompt()
	for _, expected := range []string{
		"Deliver the task itself first",
		"Goal determines what should keep being pursued",
		"These primitives are optional choices, not a mandatory pipeline",
		"minimum structure whose value exceeds its coordination cost",
		"load the `execution-orchestrator` Skill",
		"progressively discloses only the relevant strategy",
		"Graph UI already display",
		"`allowed_actions`",
		"Use structured tools to record responsibility and state transitions",
		"Bridge lifecycle observation records actual Tool and Subagent runs",
		"never stop merely to ask the user to send “continue”",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("stable execution prompt missing %q", expected)
		}
	}
	if words := len(strings.Fields(prompt)); words > 300 {
		t.Fatalf("stable execution prompt has %d words, want at most 300", words)
	}
	for _, proceduralDetail := range []string{
		"native `items` object array",
		"`return_to_agent_id`",
		"with or without a following space",
		"`audit_execution_alignment`",
	} {
		if strings.Contains(prompt, proceduralDetail) {
			t.Fatalf("stable execution prompt leaked skill/tool detail %q", proceduralDetail)
		}
	}
}
