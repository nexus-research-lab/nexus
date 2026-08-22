package dm

import (
	"slices"
	"testing"
)

func TestWorkGraphDistillationPolicyAllowsInputSlotRead(t *testing.T) {
	policy := workGraphDistillationRuntimePolicy()
	if !slices.Contains(policy.ToolPolicy.AllowedTools, "Read") ||
		slices.Contains(policy.ToolPolicy.DisallowedTools, "Read") {
		t.Fatalf("distillation must support the CLI input-slot Read→Write contract: %#v", policy.ToolPolicy)
	}
}
