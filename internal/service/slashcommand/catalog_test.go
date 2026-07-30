package slashcommand

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestCatalogReturnsVersionedRuntimeManifests(t *testing.T) {
	catalog := NewCatalog()
	tests := []struct {
		kind         agentclient.RuntimeKind
		wantCommands []string
	}{
		{
			kind:         agentclient.RuntimeNXS,
			wantCommands: []string{"compact", "model", "skills", "summary"},
		},
		{
			kind: agentclient.RuntimeClaude,
			wantCommands: []string{
				"compact",
				"model",
				"skills",
			},
		},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			snapshot := catalog.Snapshot(test.kind)
			if snapshot.Status != protocol.CommandCatalogStatusReady ||
				snapshot.Generation != catalogGeneration ||
				snapshot.RuntimeKind != test.kind ||
				len(snapshot.Commands) != len(test.wantCommands) {
				t.Fatalf("Snapshot(%q) = %#v", test.kind, snapshot)
			}
			for index, command := range snapshot.Commands {
				if command.Name != test.wantCommands[index] ||
					command.Execution != protocol.CommandExecutionRuntime ||
					!command.Enabled {
					t.Fatalf(
						"Snapshot(%q).Commands[%d] = %#v",
						test.kind,
						index,
						command,
					)
				}
			}
		})
	}
}

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
