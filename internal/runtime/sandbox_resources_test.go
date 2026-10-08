package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if marker.ProcessStartTimeUnixNano < 0 {
		t.Fatalf("durable marker has a negative process start time: %#v", marker)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker remains after release: %v", err)
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
	if first == second || first.Path() != second.Path() {
		t.Fatalf("active session did not return an independent handle: first=%p second=%p paths=%q/%q", first, second, first.Path(), second.Path())
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.Path()); err != nil {
		t.Fatalf("releasing one active-session handle removed the shared resource: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shared resource remains after final handle release: %v", err)
	}
}

func TestConcurrentAcquireDoesNotTreatPublishingMarkerAsCrash(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	results := make(chan *Lease, 16)
	errorsCh := make(chan error, 16)
	start := make(chan struct{})
	for range 16 {
		wg.Go(func() {
			<-start
			lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "concurrent-acquire", Root: root})
			if err != nil {
				errorsCh <- err
				return
			}
			results <- lease
		})
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)
	var path string
	for lease := range results {
		if path != "" && path != lease.Path() {
			t.Errorf("concurrent startup published a second scratch: %q != %q", path, lease.Path())
		}
		path = lease.Path()
		if err := lease.Release(); err != nil {
			t.Error(err)
		}
	}
	for err := range errorsCh {
		t.Errorf("concurrent scratch acquisition: %v", err)
	}
}

func TestReleasedLeaseCannotExposeOrMutateSandbox(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "released-access", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if lease.Path() != "" || lease.Resources() != nil || lease.Marker() != nil || lease.RoundID() != "" {
		t.Fatalf("released lease still exposed state: path=%q resources=%#v marker=%#v round=%q", lease.Path(), lease.Resources(), lease.Marker(), lease.RoundID())
	}
	if err := lease.MarkCleanupUncertain(errors.New("late cleanup")); err != nil {
		t.Fatalf("released lease cleanup marker mutation = %v", err)
	}
}

func TestLeaseReleaseSerializesConcurrentCallsOnOneHandle(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "concurrent-release", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "concurrent-release", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	path := first.Path()

	var wg sync.WaitGroup
	results := make(chan error, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- first.Release()
		}()
	}
	wg.Wait()
	close(results)
	for releaseErr := range results {
		if releaseErr != nil {
			t.Fatalf("concurrent release failed: %v", releaseErr)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("one independent lease was removed by duplicate release: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final independent lease did not remove resource: %v", err)
	}
}

func TestSandboxResourceCleanupFailureFencesNewAcquisition(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "fenced", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "fenced", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	first.MarkCleanupUncertain(errors.New("bridge descendants remain"))
	if _, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "fenced", Root: root}); err == nil {
		t.Fatal("poisoned sandbox resource accepted a new acquisition")
	}
	if err := second.Release(); err != nil {
		t.Fatalf("independent preparation handle could not be released: %v", err)
	}
	if _, err := os.Stat(first.Path()); err != nil {
		t.Fatalf("fenced resource disappeared while uncertain runtime was retained: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("reconciled lease release = %v", err)
	}
}

func TestUncertainLeaseReleaseTransfersCleanupFenceToSibling(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "uncertain-transfer", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "uncertain-transfer", Root: root})
	if err != nil {
		_ = first.Release()
		t.Fatal(err)
	}
	path := first.Path()
	if err := first.MarkCleanupUncertain(errors.New("bridge descendants remain")); err != nil {
		t.Fatal(err)
	}
	// Releasing the handle that first observed uncertainty must not strand the
	// fence on an already-idempotent handle. The sibling remains the exact
	// owner allowed to perform the eventual cleanup.
	if err := first.Release(); err != nil {
		t.Fatalf("uncertain owner release = %v", err)
	}
	if _, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "uncertain-transfer", Root: root}); err == nil {
		t.Fatal("cleanup-uncertain resource accepted a replacement runtime")
	}
	if err := second.Release(); err != nil {
		t.Fatalf("sibling release after transferred cleanup fence = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resource remains after sibling reconciled cleanup: %v", err)
	}
}

func TestAcquireRejectsWriteScopeChangeForActiveSession(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(t.Context(), Input{
		OwnerUserID: "owner", SessionKey: "session", Root: root,
		WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Release() }()
	if second, err := Acquire(t.Context(), Input{
		OwnerUserID: "owner", SessionKey: "session", Root: root,
		WriteScope: agentclient.SandboxWriteScopeReadOnly,
	}); err == nil || second != nil {
		t.Fatalf("active session changed resource scope: lease=%v err=%v", second, err)
	}
	if got := first.Resources(); got == nil || got.WriteScope != agentclient.SandboxWriteScopeWorkspaceWrite {
		t.Fatalf("original lease scope changed after rejected acquire: %#v", got)
	}
}

func TestReleaseRetainsLeaseWhenScratchParentIsReplaced(t *testing.T) {
	root := t.TempDir()
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "symlink-parent", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	path := lease.Path()
	base := filepath.Dir(path)
	movedBase := base + "-moved"
	if err := os.Rename(base, movedBase); err != nil {
		t.Fatal(err)
	}
	redirect := t.TempDir()
	replacedBySymlink := true
	if err := os.Symlink(redirect, base); err != nil {
		replacedBySymlink = false
		if writeErr := os.WriteFile(base, []byte("replacement"), 0o600); writeErr != nil {
			_ = os.Rename(movedBase, base)
			t.Fatalf("replace scratch parent: symlink=%v file=%v", err, writeErr)
		}
	}
	sentinel := filepath.Join(redirect, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err == nil {
		t.Fatal("release succeeded through a replaced scratch parent")
	}
	canonicalRoot, canonicalErr := filepath.EvalSymlinks(root)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	marker, markerErr := readSandboxLeaseMarker(filepath.Join(movedBase, filepath.Base(path)), filepath.Clean(canonicalRoot), "owner")
	if markerErr != nil {
		t.Fatalf("durable cleanup marker after failed release: %v", markerErr)
	}
	if marker.CleanupState != cleanupStateUnknown || marker.CleanupUpdatedAt.IsZero() {
		t.Fatalf("failed release did not persist cleanup_unknown: %#v", marker)
	}
	if replacedBySymlink {
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("release touched redirected parent: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(movedBase, filepath.Base(path))); err != nil {
		t.Fatalf("original scratch disappeared after failed release: %v", err)
	}
	if err := os.Remove(base); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(movedBase, base); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
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

func writeSandboxLeaseMarker(path string, marker SandboxLeaseMarker) error {
	root, err := openSandboxDirectory(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeSandboxLeaseMarkerInRoot(root, marker)
}

// ReleasePath is retained as a fail-closed compatibility helper. Cleanup of a
// live resource must use the exact Lease handle; a path alone cannot identify
// which runtime generation owns a reference.
func ReleasePath(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return nil
	}
	registryMu.Lock()
	resource := registry[path]
	registryMu.Unlock()
	if resource == nil {
		return nil
	}
	return errors.New("sandbox lease cleanup requires its exact handle")
}
