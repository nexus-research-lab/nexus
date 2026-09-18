package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestAcquireCreatesPrivatePolicyAndReleaseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	lease, err := Acquire(context.Background(), Input{OwnerUserID: "owner/a", SessionKey: "dm:1", RoundID: "round/1", Root: root, WriteScope: agentclient.SandboxWriteScopeReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Dir(path)) != filepath.Clean(canonicalRoot) {
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

func TestAcquirePersistsDurableMarkerAndReleaseRemovesIt(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", RoundID: "round", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(lease.Path(), leaseMarkerName)
	payload, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	var marker SandboxLeaseMarker
	if err := json.Unmarshal(payload, &marker); err != nil {
		t.Fatal(err)
	}
	if marker.Version != leaseMarkerVersion || marker.OwnerUserID != "owner" || marker.SessionKey != "session" || marker.RoundID != "round" || marker.ProcessID != os.Getpid() || marker.RuntimeRoot != canonicalRoot || marker.LeaseID == "" || marker.CreatedAt.IsZero() {
		t.Fatalf("durable marker = %#v", marker)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker remains after release: %v", err)
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

func TestSweepStaleSandboxResourcesRequiresExplicitApply(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, scratchDirName)
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, ".scratch-crashed")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := SandboxLeaseMarker{
		Version:     leaseMarkerVersion,
		LeaseID:     "crashed-lease",
		OwnerUserID: "owner",
		SessionKey:  "session",
		RuntimeRoot: canonicalRoot,
		ProcessID:   os.Getpid() + 1000000,
		CreatedAt:   time.Now().Add(-2 * time.Hour).UTC(),
	}
	if err := writeSandboxLeaseMarker(path, marker); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	dryRun, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{OwnerUserID: "owner", Root: root, OlderThan: time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(dryRun.Candidates) != 1 || len(dryRun.Removed) != 0 {
		t.Fatalf("dry-run result = %#v", dryRun)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dry run removed stale path: %v", err)
	}
	applied, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{OwnerUserID: "owner", Root: root, OlderThan: time.Hour, Now: now, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Removed) != 1 || len(applied.Candidates) != 1 {
		t.Fatalf("apply result = %#v", applied)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale path remains after explicit apply: %v", err)
	}
}

func TestSweepStaleSandboxResourcesRetainsActiveAndMalformedMarkers(t *testing.T) {
	root := t.TempDir()
	active, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "active", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, scratchDirName)
	malformed := filepath.Join(base, ".scratch-malformed")
	if err := os.Mkdir(malformed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(malformed, leaseMarkerName), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{OwnerUserID: "owner", Root: root, OlderThan: time.Nanosecond, Now: time.Now().Add(time.Hour), Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 || len(result.Candidates) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("active/malformed sweep result = %#v", result)
	}
	if _, err := os.Stat(active.Path()); err != nil {
		t.Fatalf("active lease was removed: %v", err)
	}
	if err := active.Release(); err != nil {
		t.Fatal(err)
	}
}
