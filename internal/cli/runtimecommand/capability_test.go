package runtimecommand

import (
	"strings"
	"testing"
	"time"
)

type recordingRoundResolver struct {
	running map[string][]string
	calls   []string
}

func (r *recordingRoundResolver) GetRunningRoundIDs(sessionKey string) []string {
	r.calls = append(r.calls, sessionKey)
	return append([]string(nil), r.running[sessionKey]...)
}

func TestCapabilityResolveUsesOneRoundLookupAndFailsClosedOnConcurrency(t *testing.T) {
	resolver := &recordingRoundResolver{running: map[string][]string{}}
	registry := NewRegistry(resolver)
	first := testCapabilityActor("round-1")
	second := testCapabilityActor("round-2")
	firstToken, err := registry.Issue(first)
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := registry.Issue(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstToken != secondToken {
		t.Fatalf("one runtime session received different capability tokens: %q != %q", firstToken, secondToken)
	}

	resolver.running[first.LeaseSessionKey] = []string{first.LeaseRoundID, first.LeaseRoundID}
	resolved, err := registry.Resolve(firstToken)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.RoundID != first.RoundID {
		t.Fatalf("resolved round = %q, want %q", resolved.RoundID, first.RoundID)
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != first.LeaseSessionKey {
		t.Fatalf("round resolver calls = %#v, want one lookup", resolver.calls)
	}

	resolver.running[first.LeaseSessionKey] = []string{first.LeaseRoundID, second.LeaseRoundID}
	if _, err = registry.Resolve(firstToken); err == nil || !strings.Contains(err.Error(), "并发 round") {
		t.Fatalf("concurrent resolve error = %v", err)
	}
	if len(resolver.calls) != 2 {
		t.Fatalf("concurrent resolve used %d total lookups, want 2", len(resolver.calls))
	}
}

func TestCapabilityResolveLazilyExpiresItsToken(t *testing.T) {
	resolver := &recordingRoundResolver{running: map[string][]string{"runtime-session": {"round-1"}}}
	registry := NewRegistry(resolver)
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	registry.now = func() time.Time { return now }
	token, err := registry.Issue(testCapabilityActor("round-1"))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(capabilityTTL)
	if _, err = registry.Resolve(token); err == nil {
		t.Fatal("expired capability resolved successfully")
	}
	if _, exists := registry.records[token]; exists {
		t.Fatal("expired capability record was not removed")
	}
}

func testCapabilityActor(roundID string) Actor {
	return Actor{
		OwnerUserID: "owner", AgentID: "agent", SessionKey: "conversation",
		RoundID: roundID, LeaseSessionKey: "runtime-session", LeaseRoundID: roundID,
		Round: RoundContext{Receipts: NewReceiptState()},
	}
}
