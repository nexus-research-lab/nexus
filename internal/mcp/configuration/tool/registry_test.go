package tool

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
)

func TestBuildAllExposesStableConfigurationWorkflow(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{})
	if len(tools) != 4 {
		t.Fatalf("tool count = %d, want 4", len(tools))
	}
	names := []string{
		"inspect_nexus_configuration",
		"plan_nexus_configuration_change",
		"apply_nexus_configuration_change",
		"list_nexus_configuration_changes",
	}
	for index, name := range names {
		if tools[index].Name != name {
			t.Fatalf("tool[%d] = %q, want %q", index, tools[index].Name, name)
		}
	}
	if !tools[0].AlwaysLoad || tools[0].Annotations == nil || !tools[0].Annotations.ReadOnly {
		t.Fatal("inspect tool must be always loaded and read-only")
	}
	if tools[2].Annotations == nil || !tools[2].Annotations.Destructive {
		t.Fatal("apply tool must carry destructive permission annotation")
	}
	if !tools[2].Annotations.OpenWorld {
		t.Fatal("apply tool may explicitly test providers and must carry open-world annotation")
	}
}
