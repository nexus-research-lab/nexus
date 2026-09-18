package sandboxresources

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestAcquireCreatesPrivatePolicyAndReleaseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	lease, err := Acquire(context.Background(), Input{OwnerUserID: "owner/a", SessionKey: "dm:1", RoundID: "round/1", Root: root, WriteScope: agentclient.SandboxWriteScopeReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Dir(path)) != filepath.Clean(root) {
		t.Fatalf("scratch path = %q", path)
	}
	if got := lease.Resources(); got == nil || got.Version != 1 || got.WriteScope != agentclient.SandboxWriteScopeReadOnly || got.ScratchRoot != path {
		t.Fatalf("resources = %#v", lease.Resources())
	}
	if err := os.WriteFile(filepath.Join(path, "output.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch remains after release: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestReleasePathDoesNotDeleteUnregisteredDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".scratch-unregistered")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ReleasePath(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireRejectsInvalidScopeAndCancellation(t *testing.T) {
	if _, err := Acquire(context.Background(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir(), WriteScope: "bad"}); err == nil {
		t.Fatal("invalid scope accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Acquire(ctx, Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquire error = %v", err)
	}
}

func TestAcquireReusesActiveSessionLease(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", RoundID: "round-1", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", RoundID: "round-2", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.Path() != second.Path() {
		t.Fatalf("active session lease was replaced: first=%p second=%p paths=%q/%q", first, second, first.Path(), second.Path())
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}
