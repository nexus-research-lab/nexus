package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// TestAgentClientReceiptRetryRetainsLeaseDuringReconfigure verifies that a
// receipt failure from an obsolete startup does not consume the host lease
// before the connect flight retries with the newly requested options.
func TestAgentClientReceiptRetryRetainsLeaseDuringReconfigure(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "receipt-reconfigure",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	started := make(chan struct{})
	release := make(chan struct{})
	want := errors.New("retry stopped")
	var opened atomic.Int32
	client := &agentClient{
		options: bridge.Options{
			Model:   "old",
			Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS},
			Env:     map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"},
		},
		sandboxLease: lease,
		closeSession: func(*bridge.Session) error { return nil },
	}
	client.newSession = func(context.Context, bridge.Options) (*bridge.Session, error) {
		switch opened.Add(1) {
		case 1:
			close(started)
			<-release
			// The desktop NXS options intentionally omit the complete sandbox
			// contract, so receipt validation fails after this session opens.
			return &bridge.Session{}, nil
		case 2:
			if got := client.currentSandboxLease(); got != lease {
				t.Errorf("retry lost original sandbox lease: got %p want %p", got, lease)
			}
			return nil, want
		default:
			return nil, errors.New("unexpected additional startup")
		}
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(ctx) }()
	<-started

	updated := bridge.Options{
		Model:   "new",
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS},
		Env:     map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"},
	}
	if err := client.Reconfigure(ctx, updated); err != nil {
		t.Fatalf("Reconfigure() = %v", err)
	}
	close(release)
	if err := <-connectDone; !errors.Is(err, want) {
		t.Fatalf("Connect() = %v, want retry failure %v", err, want)
	}
	if opened.Load() != 2 {
		t.Fatalf("opened %d sessions, want obsolete startup plus one retry", opened.Load())
	}
	if got := client.currentSandboxLease(); got != lease {
		t.Fatalf("client lease after receipt retry = %p, want original %p", got, lease)
	}
}

func TestAgentClientStartupReconfigureRetainsLeaseDuringStaleCleanup(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "stale-startup-reconfigure",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	started := make(chan struct{})
	release := make(chan struct{})
	want := errors.New("latest startup failed")
	var opened atomic.Int32
	client := &agentClient{
		options:      bridge.Options{Model: "old", Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}},
		sandboxLease: lease,
		closeSession: func(*bridge.Session) error { return nil },
	}
	client.newSession = func(context.Context, bridge.Options) (*bridge.Session, error) {
		switch opened.Add(1) {
		case 1:
			close(started)
			<-release
			return &bridge.Session{}, nil
		case 2:
			if got := client.currentSandboxLease(); got != lease {
				t.Errorf("retry lost original sandbox lease: got %p want %p", got, lease)
			}
			return nil, want
		default:
			return nil, errors.New("unexpected additional startup")
		}
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(ctx) }()
	<-started
	updated := bridge.Options{Model: "new", Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}}
	if err := client.Reconfigure(ctx, updated); err != nil {
		t.Fatalf("Reconfigure() = %v", err)
	}
	close(release)
	if err := <-connectDone; !errors.Is(err, want) {
		t.Fatalf("Connect() = %v, want retry failure %v", err, want)
	}
	if opened.Load() != 2 {
		t.Fatalf("opened %d sessions, want obsolete startup plus one retry", opened.Load())
	}
	if got := client.currentSandboxLease(); got != lease {
		t.Fatalf("client lease after stale cleanup = %p, want original %p", got, lease)
	}
}

func TestAgentClientDiscardDuringConnectClosesLateSessionBeforeReleasingLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "late-session-discard",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	defer lease.Release()

	started := make(chan struct{})
	release := make(chan struct{})
	closed := make(chan error, 1)
	client := &agentClient{
		options:      bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}},
		sandboxLease: lease,
		closeSession: func(session *bridge.Session) error {
			if session == nil {
				return errors.New("late session was not passed to cleanup")
			}
			if _, statErr := os.Stat(path); statErr != nil {
				return fmt.Errorf("lease released before late session close: %w", statErr)
			}
			closed <- nil
			return nil
		},
	}
	client.newSession = func(context.Context, bridge.Options) (*bridge.Session, error) {
		close(started)
		<-release
		return &bridge.Session{}, nil
	}

	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(t.Context()) }()
	<-started
	client.DiscardUncleanSession()
	close(release)
	if err := <-connectDone; !errors.Is(err, bridge.ErrAborted) {
		t.Fatalf("Connect() = %v, want aborted stale startup", err)
	}
	client.mu.Lock()
	cleanup := client.cleanup
	client.mu.Unlock()
	if cleanup == nil {
		t.Fatal("late session cleanup fence is missing")
	}
	<-cleanup.done
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("late session cleanup did not run")
	}
	// Connect waits for the stale candidate cleanup fence, so the successful
	// close and exact lease release are complete at this point.
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("late session lease remains after successful close: %v", statErr)
	}
}

func TestSandboxPolicyDigestIncludesHostRequirementsAndResources(t *testing.T) {
	options := bridge.Options{
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS},
		Env:     map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"},
		Sandbox: &bridge.SandboxSettings{RequireSandbox: true},
	}
	base, err := sandboxPolicyDigest(options)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*bridge.SandboxSettings){
		"file tools":      func(settings *bridge.SandboxSettings) { settings.RequireFileTools = true },
		"search tools":    func(settings *bridge.SandboxSettings) { settings.RequireSearchTools = true },
		"settings writes": func(settings *bridge.SandboxSettings) { settings.RequireSettingsWrites = true },
		"resource policy": func(settings *bridge.SandboxSettings) {
			settings.Resources = &bridge.SandboxResourcePolicy{
				Version:     1,
				WriteScope:  bridge.SandboxWriteScopeWorkspaceWrite,
				ScratchRoot: "/tmp/nexus-scratch",
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := options
			settings := *options.Sandbox
			candidate.Sandbox = &settings
			mutate(candidate.Sandbox)
			got, err := sandboxPolicyDigest(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if got == base {
				t.Fatalf("digest did not change for %s: %q", name, got)
			}
		})
	}
}

func TestRequiredSandboxCapabilitiesNormalizeRuntimeKindAliases(t *testing.T) {
	options := bridge.Options{
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeKind("go-native")},
		Sandbox: &bridge.SandboxSettings{RequireSandbox: true, RequireFileTools: true},
	}
	capabilities := requiredSandboxCapabilities(options)
	if len(capabilities) != 2 || capabilities[0] != bridge.CapabilityRequiredSandbox || capabilities[1] != bridge.CapabilitySandboxFileTools {
		t.Fatalf("required capabilities = %#v, want nxs sandbox and file tools", capabilities)
	}
	if got := receiptRuntimeKind(bridge.RuntimeKind("claude-code")); got != bridge.RuntimeClaude {
		t.Fatalf("receipt runtime kind = %q, want claude", got)
	}
}

func TestDesktopNxsReceiptRejectsWeakSandboxContract(t *testing.T) {
	options := bridge.Options{
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeKind("go-native")},
		Sandbox: &bridge.SandboxSettings{RequireSandbox: true},
	}
	if err := validateDesktopSandboxReceiptOptions(options); err == nil {
		t.Fatal("weak desktop nxs sandbox contract was accepted")
	}
	enabled := true
	complete := &bridge.SandboxSettings{
		RequireSandbox: true, RequireFileTools: true, RequireSearchTools: true,
		RequireMediaFiles: true, RequireMediaNetwork: true, RequireMCPNetwork: true, RequireNotebookFiles: true, RequireSkillFiles: true,
		RequireContextFiles: true, RequireProjectFiles: true,
		RequireManagedPolicy: true, RequireSettingsFiles: true,
		RequireSettingsWrites: true, Enabled: &enabled, FailIfUnavailable: &enabled,
	}
	if err := validateDesktopSandboxReceiptOptions(bridge.Options{
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Sandbox: complete, MCP: bridge.MCPOptions{StrictConfig: true},
	}); err != nil {
		t.Fatalf("complete desktop nxs sandbox contract rejected: %v", err)
	}
}

func TestAgentClientBindSandboxLeaseRejectsReleasedLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "released-lease",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	client := &agentClient{options: bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}}}
	if err := client.BindSandboxLease(lease); err == nil || !strings.Contains(err.Error(), "already released") {
		t.Fatalf("BindSandboxLease() = %v, want released-lease rejection", err)
	}
}

func TestAgentClientBindSandboxLeaseRejectsDifferentResourceForActiveSession(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "active-session",
		Root:        t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	client := &agentClient{
		options: bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}},
		session: &bridge.Session{},
	}
	if err := client.BindSandboxLease(lease); err == nil || !strings.Contains(err.Error(), "cannot change while runtime generation is active") {
		t.Fatalf("BindSandboxLease() = %v, want active-generation rejection", err)
	}
}

func TestAgentClientBindSandboxLeaseRefreshesSharedResourceRound(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "round-refresh",
		RoundID:     "round-a",
		Root:        root,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{
		OwnerUserID: "receipt-owner",
		SessionKey:  "round-refresh",
		RoundID:     "round-b",
		Root:        root,
	})
	if err != nil {
		_ = first.Release()
		t.Fatal(err)
	}
	defer second.Release()
	resources := first.Resources()
	if resources == nil {
		_ = first.Release()
		t.Fatal("first lease has no resource policy")
	}
	client := &agentClient{
		options: bridge.Options{
			Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS},
			Env:     map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"},
			Sandbox: &bridge.SandboxSettings{RequireSandbox: true, RequireFileTools: true, Resources: resources},
		},
		session:      &bridge.Session{},
		sandboxLease: first,
		sandboxReceipt: &SandboxEffectivePolicyReceipt{
			LeaseID: first.Marker().LeaseID,
			RoundID: "round-a",
		},
	}
	if err := client.BindSandboxLease(second); err != nil {
		t.Fatalf("BindSandboxLease() = %v", err)
	}
	if got := client.sandboxReceipt.RoundID; got != "round-b" {
		t.Fatalf("receipt round ID = %q, want round-b", got)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestEffectiveSandboxPolicyReceiptKeepsClaudeIdentityProvisional(t *testing.T) {
	client := &agentClient{
		session: &bridge.Session{},
		sandboxReceipt: &SandboxEffectivePolicyReceipt{
			RuntimeKind:          bridge.RuntimeClaude,
			SessionIDProvisional: true,
			CapabilityEvidence:   "bridge_negotiated",
			IsolationEvidence:    "not_attested",
		},
	}
	receipt := client.EffectiveSandboxPolicyReceipt()
	if receipt == nil || !receipt.SessionIDProvisional || receipt.SessionID != "" {
		t.Fatalf("receipt = %#v, want empty provisional Claude identity", receipt)
	}
}

func TestSandboxReceiptUsesExactSharedLeaseRound(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "receipt-owner", SessionKey: "shared-round", RoundID: "round-first", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "receipt-owner", SessionKey: "shared-round", RoundID: "round-second", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	defer second.Release()
	client := &agentClient{sandboxReceipt: &SandboxEffectivePolicyReceipt{LeaseID: "old", RoundID: "old"}}
	client.mu.Lock()
	client.refreshSandboxReceiptLeaseLocked(second)
	client.mu.Unlock()
	if client.sandboxReceipt.LeaseID == "" || client.sandboxReceipt.RoundID != "round-second" {
		t.Fatalf("receipt = %#v, want exact second handle round", client.sandboxReceipt)
	}
}
