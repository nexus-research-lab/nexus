package orchestration

import (
	"strings"
	"testing"
)

func TestStablePromptDefinesExecutionBoundaries(t *testing.T) {
	prompt := StablePrompt()
	for _, expected := range []string{
		"Deliver the task first",
		"Goal, Plan, WorkGraph, Room Assignment, Task/Todo, and subagents are optional",
		"smallest structure justified by",
		"Goal and WorkGraph are independent",
		"create Goal first when both are needed",
		"Before substantial execution, assess separability",
		"Use subagents only when benefit exceeds",
		"the parent integrates, verifies, and delivers",
		"`<nexus_execution_context>` is authoritative",
		"load `execution-orchestrator`",
		"Graph UI show lifecycle state",
		"`allowed_actions`",
		"round-scoped contract",
		"never ask for “continue”",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("stable execution prompt missing %q", expected)
		}
	}
	if chars := len([]rune(prompt)); chars > 900 {
		t.Fatalf("stable execution prompt has %d characters, want at most 900", chars)
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
