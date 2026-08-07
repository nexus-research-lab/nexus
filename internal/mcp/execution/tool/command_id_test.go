package tool

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func TestCommandIDPrefersToolUseID(t *testing.T) {
	got, err := commandID(
		contract.ServerContext{ScopeSessionKey: "scope"},
		&sdktool.CallContext{ToolUseID: " tool-call-42 "},
		"assign_work",
		map[string]any{"logical_key": "research"},
		7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "tool-call-42" {
		t.Fatalf("command id = %q, want tool-call-42", got)
	}
}

func TestCommandIDFallbackIsCanonicalAndRevisionFenced(t *testing.T) {
	sctx := contract.ServerContext{
		ScopeSessionKey:   "room:scope",
		RuntimeSessionKey: "runtime:agent",
		RootRoundID:       "root-3",
		AgentRoundID:      "agent-8",
	}
	first, err := commandID(
		sctx,
		nil,
		"assign_work",
		map[string]any{"target_agent_id": "a2", "logical_key": "research"},
		7,
	)
	if err != nil {
		t.Fatal(err)
	}
	same, err := commandID(
		sctx,
		&sdktool.CallContext{},
		"assign_work",
		map[string]any{"logical_key": "research", "target_agent_id": "a2"},
		7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first != same {
		t.Fatalf("map insertion order changed command id: %q != %q", first, same)
	}
	nextRevision, err := commandID(
		sctx,
		nil,
		"assign_work",
		map[string]any{"logical_key": "research", "target_agent_id": "a2"},
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first == nextRevision {
		t.Fatalf("revision fencing did not change fallback command id: %q", first)
	}
	otherRound := sctx
	otherRound.AgentRoundID = "agent-9"
	nextRound, err := commandID(
		otherRound,
		nil,
		"assign_work",
		map[string]any{"logical_key": "research", "target_agent_id": "a2"},
		7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first == nextRound {
		t.Fatalf("agent round did not change fallback command id: %q", first)
	}
}
