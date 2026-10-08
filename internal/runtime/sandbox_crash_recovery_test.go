package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	sandboxCrashHelperEnv = "NEXUS_SANDBOX_CRASH_HELPER"
	sandboxCrashRootEnv   = "NEXUS_SANDBOX_CRASH_ROOT"
	sandboxCrashStateEnv  = "NEXUS_SANDBOX_CRASH_STATE"
)

// TestSandboxCrashHelper is launched by TestSandboxCrashRestartRequiresExplicitReconcile
// in a separate process. It deliberately exits without releasing its lease so
// the parent can exercise the same durable-marker path a host restart sees.
func TestSandboxCrashHelper(t *testing.T) {
	if os.Getenv(sandboxCrashHelperEnv) != "1" || strings.TrimSpace(os.Getenv(sandboxCrashRootEnv)) == "" {
		return
	}
	root := os.Getenv(sandboxCrashRootEnv)
	if root == "" {
		t.Fatal("crash helper root is empty")
	}
	lease, err := Acquire(context.Background(), Input{
		OwnerUserID: "owner",
		SessionKey:  "crashed-session",
		RoundID:     "crashed-round",
		Root:        root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if lease.Path() == "" || lease.Marker() == nil {
		t.Fatal("crash helper did not create a durable lease")
	}
	if os.Getenv(sandboxCrashStateEnv) == cleanupStateUnknown {
		if err := lease.MarkCleanupUncertain(errors.New("helper descendants remain")); err != nil {
			t.Fatal(err)
		}
	}
	// Do not defer Release: os.Exit is the simulated host crash boundary.
	os.Exit(0)
}

// TestSandboxCrashRestartRequiresExplicitReconcile verifies the restart
// contract end to end: a crashed host leaves a marker, a fresh host discovers
// it without adopting or deleting it, and only an explicit reconcile may
// remove it. On platforms where process liveness is intentionally unknown,
// the fresh host retains the marker as a protected skip.
func TestSandboxCrashRestartRequiresExplicitReconcile(t *testing.T) {
	root := t.TempDir()
	runSandboxCrashHelper(t, root, cleanupStateActive)

	// The parent process is the restarted host. Its registry never contained
	// the child lease, so discovery must use only durable marker evidence.
	records, err := DiscoverSandboxResources(t.Context(), SandboxResourceSweepInput{
		OwnerUserID: "owner",
		Root:        root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("restart discovery = %#v, want one durable marker", records)
	}
	record := records[0]
	if record.Marker.SessionKey != "crashed-session" || record.Marker.RoundID != "crashed-round" {
		t.Fatalf("restart marker = %#v", record.Marker)
	}
	if record.Marker.CleanupState != cleanupStateActive {
		t.Fatalf("crashed marker cleanup state = %q, want active", record.Marker.CleanupState)
	}
	if _, err := os.Stat(record.Path); err != nil {
		t.Fatalf("restart discovery removed crashed scratch: %v", err)
	}

	// Reconciliation is explicitly age based. Move the observation clock ahead
	// of the marker without changing the durable record, making the fixture
	// deterministic and avoiding a sleep in the test.
	now := time.Now().UTC().Add(2 * time.Hour)
	dryRun, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{
		OwnerUserID: "owner",
		Root:        root,
		OlderThan:   time.Hour,
		Now:         now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dryRun.Removed) != 0 {
		t.Fatalf("restart dry run removed a resource: %#v", dryRun)
	}
	if _, err := os.Stat(record.Path); err != nil {
		t.Fatalf("restart dry run removed crashed scratch: %v", err)
	}

	if runtime.GOOS == "plan9" || runtime.GOOS == "wasip1" || runtime.GOOS == "js" {
		if len(dryRun.Candidates) != 0 || len(dryRun.Skipped) != 1 || !dryRun.Skipped[0].ProcessActive {
			t.Fatalf("unknown-liveness restart result = %#v", dryRun)
		}
		return
	}
	if len(dryRun.Candidates) != 1 || len(dryRun.Skipped) != 0 || dryRun.Candidates[0].ProcessActive {
		t.Fatalf("dead-process restart result = %#v", dryRun)
	}

	// Deletion is only reachable through an explicit apply=true reconcile.
	applied, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{
		OwnerUserID: "owner",
		Root:        root,
		OlderThan:   time.Hour,
		Now:         now,
		Apply:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Removed) != 1 || len(applied.Candidates) != 1 || len(applied.Skipped) != 0 {
		t.Fatalf("explicit restart reconcile result = %#v", applied)
	}
	if _, err := os.Stat(record.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("explicit reconcile did not remove crashed scratch: %v", err)
	}
}

func TestSandboxCleanupUnknownSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	runSandboxCrashHelper(t, root, cleanupStateUnknown)

	// cleanup_unknown is a durable fail-closed statement. A restarted host may
	// inspect it, but neither dead-PID evidence nor an explicit sweep may turn
	// that uncertainty into permission to remove the scratch directory.
	records, err := DiscoverSandboxResources(t.Context(), SandboxResourceSweepInput{
		OwnerUserID: "owner",
		Root:        root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("cleanup_unknown restart discovery = %#v, want one marker", records)
	}
	record := records[0]
	if record.Marker.CleanupState != cleanupStateUnknown || !record.ProcessActive {
		t.Fatalf("cleanup_unknown restart marker = %#v", record)
	}

	now := time.Now().UTC().Add(2 * time.Hour)
	result, err := SweepStaleSandboxResources(t.Context(), SandboxResourceSweepInput{
		OwnerUserID: "owner",
		Root:        root,
		OlderThan:   time.Hour,
		Now:         now,
		Apply:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || len(result.Removed) != 0 || len(result.Skipped) != 1 || !result.Skipped[0].ProcessActive {
		t.Fatalf("cleanup_unknown restart reconcile = %#v", result)
	}
	if _, err := os.Stat(record.Path); err != nil {
		t.Fatalf("cleanup_unknown reconcile removed scratch: %v", err)
	}
	// A new scratch path must not bypass the durable close failure after a
	// restart. The background-memory runner also enters through Acquire.
	for _, pathVariant := range []string{"original", "legacy_replacement"} {
		t.Run(pathVariant, func(t *testing.T) {
			if pathVariant == "legacy_replacement" {
				if err := os.Rename(record.Path, record.Path+"-stale-legacy"); err != nil {
					t.Fatal(err)
				}
			}
			for _, scope := range []agentclient.SandboxWriteScope{agentclient.SandboxWriteScopeWorkspaceWrite, agentclient.SandboxWriteScopeReadOnly} {
				lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "crashed-session", RoundID: "later-round", Root: root, WriteScope: scope})
				if err == nil {
					_ = lease.Release()
					t.Fatalf("restart bypassed cleanup_unknown with write scope %s", scope)
				}
			}
		})
	}
	other, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "independent-session", Root: root})
	if err != nil {
		t.Fatalf("unrelated session was blocked: %v", err)
	}
	if err := other.Release(); err != nil {
		t.Fatal(err)
	}
}

func runSandboxCrashHelper(t *testing.T, root, state string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSandboxCrashHelper$")
	cmd.Env = sandboxCrashChildEnv(root, state)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("crash helper failed: %v\n%s", err, output)
	}
}

func sandboxCrashChildEnv(root, state string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		key := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			key = item[:index]
		}
		switch key {
		case sandboxCrashHelperEnv, sandboxCrashRootEnv, sandboxCrashStateEnv:
			continue
		}
		env = append(env, item)
	}
	return append(env,
		sandboxCrashHelperEnv+"=1",
		sandboxCrashRootEnv+"="+root,
		sandboxCrashStateEnv+"="+state,
	)
}
