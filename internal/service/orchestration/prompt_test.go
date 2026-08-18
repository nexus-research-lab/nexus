package orchestration

import (
	"strings"
	"testing"
)

func TestStablePromptDefinesExecutionBoundaries(t *testing.T) {
	prompt := StablePrompt()
	for _, expected := range []string{
		"Deliver the task itself first",
		"Goal determines what persists",
		"Choose Goal and WorkGraph independently",
		"neither implies the other",
		"create the requested Goal before preparing its WorkGraph",
		"never in parallel",
		"Before substantial execution, every Agent assesses atomicity",
		"Use native subagents only when their benefit exceeds",
		"the parent integrates, verifies, and delivers",
		"Parallel execution requires distinct live contexts",
		"concurrent Work Items to different Room Agents",
		"keep one Work Item and use separate native subagents",
		"form a serial queue unless child subagents run",
		"call them queued, not parallel",
		"These primitives are optional, not a mandatory pipeline",
		"Add only structure whose value exceeds coordination cost",
		"Complexity and participant count trigger assessment",
		"not the word “collaborate” or `@`",
		"pre-materialization `assign_work` denial means finish bootstrap",
		"load the `execution-orchestrator` Skill",
		"Graph UI already show lifecycle state",
		"`allowed_actions`",
		`"${NEXUS_COMMAND_PATH}" --json execution contract|inspect|invoke`,
		"Action names are semantic operations, not tool-schema or MCP names",
		"Never use nexusctl, the retired Execution MCP",
		"Bridge observation records actual Tool and Subagent runs",
		"never ask the user to send “continue”",
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
