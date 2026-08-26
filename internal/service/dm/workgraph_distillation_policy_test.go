package dm

import (
	"slices"
	"testing"
)

func TestWorkGraphDistillationPolicyUsesStructuredRuntimeCommand(t *testing.T) {
	policy := workGraphDistillationRuntimePolicy()
	if !slices.Contains(policy.ToolPolicy.AllowedTools, "mcp__nexus__execution_write") ||
		slices.Contains(policy.ToolPolicy.AllowedTools, "Read") ||
		slices.Contains(policy.ToolPolicy.AllowedTools, "Write") {
		t.Fatalf("distillation must use the structured runtime command without file staging: %#v", policy.ToolPolicy)
	}
}
