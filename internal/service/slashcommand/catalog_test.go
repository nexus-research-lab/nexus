package slashcommand

import (
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
