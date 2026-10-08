package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestManagerSandboxPolicyReceiptIsOwnerScopedAndCopied(t *testing.T) {
	manager := NewManager()
	client := &agentClient{sandboxReceipt: &SandboxEffectivePolicyReceipt{
		Version:            sandboxEffectivePolicyReceiptVersion,
		RuntimeKind:        bridge.RuntimeNXS,
		SessionID:          "session-id",
		PolicyDigest:       "sha256:policy",
		CapabilityEvidence: "bridge_negotiated",
		IsolationEvidence:  "not_attested",
	}}
	manager.sessions["session-key"] = &sessionState{
		OwnerUserID: "owner",
		Client:      client,
	}

	receipt := manager.SandboxPolicyReceipt("owner", "session-key")
	if receipt == nil || receipt.SessionID != "session-id" {
		t.Fatalf("receipt = %#v, want current owner receipt", receipt)
	}
	receipt.SessionID = "mutated"
	if got := client.EffectiveSandboxPolicyReceipt().SessionID; got != "session-id" {
		t.Fatalf("manager returned mutable receipt: client session ID = %q", got)
	}
	if got := manager.SandboxPolicyReceipt("other", "session-key"); got != nil {
		t.Fatalf("cross-owner lookup returned %#v", got)
	}
	manager.sessions["session-key"].Closing = true
	if got := manager.SandboxPolicyReceipt("owner", "session-key"); got != nil {
		t.Fatalf("closing session returned %#v", got)
	}
}

type recordingSandboxReceiptStore struct {
	phase      protocol.SandboxPolicyReceiptPhase
	reason     string
	owner      string
	session    string
	generation uint64
	updates    int
	latest     protocol.SandboxPolicyReceiptSnapshot
	found      bool
}

func (s *recordingSandboxReceiptStore) Save(context.Context, protocol.SandboxPolicyReceiptSnapshot) error {
	return nil
}

func (s *recordingSandboxReceiptStore) UpdatePhase(_ context.Context, owner, session string, generation uint64, phase protocol.SandboxPolicyReceiptPhase, reason string) error {
	s.owner, s.session, s.generation = owner, session, generation
	s.phase, s.reason, s.updates = phase, reason, s.updates+1
	return nil
}

func (s *recordingSandboxReceiptStore) Latest(_ context.Context, owner, session string) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	if owner != s.latest.OwnerUserID || session != s.latest.SessionKey {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, nil
	}
	return s.latest, s.found, nil
}

func TestManagerSandboxReceiptPhaseUpdateUsesCapturedReceiptAfterRetire(t *testing.T) {
	store := &recordingSandboxReceiptStore{}
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	client := &agentClient{sandboxReceipt: &SandboxEffectivePolicyReceipt{SessionID: "session-id"}}
	if err := manager.updateSandboxReceiptPhase(
		"owner", "session-key", 7, client, client.sandboxReceipt,
		protocol.SandboxPolicyReceiptUnknown, "close ACK lost",
	); err != nil {
		t.Fatal(err)
	}
	if store.updates != 1 || store.owner != "owner" || store.session != "session-key" || store.generation != 7 ||
		store.phase != protocol.SandboxPolicyReceiptUnknown || store.reason != "close ACK lost" {
		t.Fatalf("store = %#v", store)
	}
}

func TestManagerReadsOwnerScopedPersistentSandboxReceipt(t *testing.T) {
	store := &recordingSandboxReceiptStore{
		latest: testSandboxReceiptSnapshotForManager(),
		found:  true,
	}
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	receipt, found, err := manager.PersistentSandboxPolicyReceipt(context.Background(), "owner-1", "session-1")
	if err != nil || !found || receipt == nil {
		t.Fatalf("receipt=%#v found=%t err=%v", receipt, found, err)
	}
	if receipt.PolicyDigest != "sha256:policy" || receipt.RoundID != "round-4" ||
		receipt.Phase != protocol.SandboxPolicyReceiptUnknown || receipt.UnknownReason != "close ACK lost" {
		t.Fatalf("receipt=%#v", receipt)
	}
	if _, found, err := manager.PersistentSandboxPolicyReceipt(context.Background(), "other-owner", "session-1"); err != nil || found {
		t.Fatalf("cross-owner read found=%t err=%v", found, err)
	}
}

func TestManagerOwnerReaperFailureDowngradesRetiredReceiptToUnknown(t *testing.T) {
	store := &recordingSandboxReceiptStore{}
	reaperErr := errors.New("owner descendants remain")
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	manager.SetOwnerProcessReaper(&fakeOwnerProcessReaper{err: reaperErr})
	client := &agentClient{sandboxReceipt: &SandboxEffectivePolicyReceipt{
		Version:            sandboxEffectivePolicyReceiptVersion,
		RuntimeKind:        bridge.RuntimeNXS,
		Generation:         9,
		PolicyDigest:       "sha256:policy",
		CapabilityEvidence: "bridge_negotiated",
		IsolationEvidence:  "not_attested",
	}}
	manager.sessions["owner-reaper-session"] = &sessionState{
		Client:               client,
		StartupGeneration:    9,
		OwnerUserID:          "owner-reaper",
		ContextUsageByAgent:  make(map[string]protocol.ContextUsageData),
		Rounds:               roundRegistry{items: make(map[string]*roundState)},
		BackgroundTasks:      make(map[uint64]context.CancelFunc),
		BackgroundDone:       closedSignal(),
		SubagentHooks:        make(map[string]SubagentHookCallbacks),
		SubagentHookBindings: make(map[string]subagentHookBinding),
	}

	closed, err := manager.CloseOwnerSessions(context.Background(), "owner-reaper")
	if closed != 1 || !errors.Is(err, reaperErr) {
		t.Fatalf("CloseOwnerSessions() = closed=%d err=%v, want one session and reaper error", closed, err)
	}
	if store.phase != protocol.SandboxPolicyReceiptUnknown ||
		!strings.Contains(store.reason, "owner process reaper failed") {
		t.Fatalf("receipt phase=%q reason=%q, want conservative unknown downgrade", store.phase, store.reason)
	}
}

func testSandboxReceiptSnapshotForManager() protocol.SandboxPolicyReceiptSnapshot {
	return protocol.SandboxPolicyReceiptSnapshot{
		Version: 1, OwnerUserID: "owner-1", SessionKey: "session-1", SessionID: "bridge-1",
		RuntimeKind: "nxs", Generation: 4, RoundID: "round-4", PolicyDigest: "sha256:policy",
		RequiredCapabilities: []string{"sandbox"}, AcknowledgedCapabilities: []string{"sandbox"},
		CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested",
		Phase: protocol.SandboxPolicyReceiptUnknown, UnknownReason: "close ACK lost",
	}
}
