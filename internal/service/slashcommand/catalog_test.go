package slashcommand

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestCatalogNormalizesAliasesAndReturnsCopies(t *testing.T) {
	catalog := NewCatalog()
	claude := catalog.Snapshot(agentclient.RuntimeKind("cc"))
	if claude.RuntimeKind != agentclient.RuntimeClaude ||
		len(claude.Commands) == 0 {
		t.Fatalf("cc snapshot = %#v, want Claude manifest", claude)
	}
	claude.Commands[0].Name = "mutated"
	if got := catalog.Snapshot(agentclient.RuntimeClaude).Commands[0].Name; got != "compact" {
		t.Fatalf("snapshot exposed mutable command list: %q", got)
	}
}

func TestCatalogRejectsUnknownRuntimeKind(t *testing.T) {
	snapshot := NewCatalog().Snapshot(agentclient.RuntimeKind("unknown"))
	if snapshot.Status != protocol.CommandCatalogStatusUnavailable ||
		snapshot.RuntimeKind != agentclient.RuntimeKind("unknown") ||
		len(snapshot.Commands) != 0 {
		t.Fatalf("unknown snapshot = %#v, want unavailable", snapshot)
	}
}

func TestVisualizeCommandExpandsOnlyRuntimePrompt(t *testing.T) {
	descriptor := VisualizeCommandDescriptor()
	if descriptor.Name != "visualize" ||
		descriptor.ArgumentHint != "<request>" ||
		descriptor.Execution != protocol.CommandExecutionRuntime ||
		!descriptor.Enabled {
		t.Fatalf("visualize descriptor = %#v", descriptor)
	}

	const raw = "/visualize quarterly revenue"
	want := "Use Generative UI to create an interactive visual for the following request:\n\nquarterly revenue"
	if got := ExpandVisualizePrompt(raw); got != want {
		t.Fatalf("ExpandVisualizePrompt(%q) = %q, want %q", raw, got, want)
	}
	if got := ExpandVisualizePrompt("/model nxs/default"); got != "/model nxs/default" {
		t.Fatalf("unrelated command changed to %q", got)
	}
}

func TestWorkGraphCommandEnablesCollaborationWithoutDistillation(t *testing.T) {
	descriptor := WorkGraphCommandDescriptor()
	if descriptor.Name != "workgraph" || descriptor.ArgumentHint != "<request>" ||
		descriptor.Execution != protocol.CommandExecutionRuntime || !descriptor.Enabled {
		t.Fatalf("workgraph descriptor = %#v", descriptor)
	}
	expanded := ExpandProductPrompt("/workgraph compare storage engines")
	if expanded == "/workgraph compare storage engines" ||
		!containsAll(expanded, "execution-orchestrator", "fresh managed WorkGraph", "compare storage engines") {
		t.Fatalf("workgraph expansion = %q", expanded)
	}
	if containsAll(expanded, "distill_workgraph_workflow") {
		t.Fatalf("fixed /workgraph acquired workflow-saving semantics: %q", expanded)
	}
}

func containsAll(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if !strings.Contains(value, candidate) {
			return false
		}
	}
	return true
}
