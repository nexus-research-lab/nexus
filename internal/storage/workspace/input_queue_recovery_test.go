package workspace

import (
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"os"
	"path/filepath"
	"testing"
)

func TestQueueRecoveryPreservesClaimOrderAndStopAcrossRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a"), SessionKey: "agent:agent-a:ws:dm:recover"}
	store := NewInputQueueStore("")
	for _, id := range []string{"first", "second"} {
		if _, err := store.Enqueue(location, protocol.InputQueueItem{ID: id, Content: id}); err != nil {
			t.Fatal(err)
		}
	}
	item, _, err := store.ClaimFirstDispatchable(location)
	if err != nil || item == nil || item.ID != "first" || item.DispatchPending {
		t.Fatalf("fresh claim: %+v %v", item, err)
	}
	store = NewInputQueueStore("")
	items, err := store.Snapshot(location)
	if err != nil || len(items) != 2 || items[0].ID != "first" || !items[0].DispatchPending {
		t.Fatalf("lost claim/order: %+v %v", items, err)
	}
	if err := store.SetDispatchPaused(location, true); err != nil {
		t.Fatal(err)
	}
	store = NewInputQueueStore("")
	item, items, err = store.ClaimFirstDispatchable(location)
	if err != nil || item != nil || !items[0].DispatchPaused {
		t.Fatalf("stop lost on restart: %+v %v", items, err)
	}
	locations, err := store.LocationsForWorkspace("owner", location.WorkspacePath)
	if err != nil || len(locations) != 1 || locations[0].SessionKey != location.SessionKey {
		t.Fatalf("queue without meta not discovered: %+v %v", locations, err)
	}
	if err := store.SetDispatchPaused(location, false); err != nil {
		t.Fatal(err)
	}
	item, _, err = store.ClaimFirstDispatchable(location)
	if err != nil || item == nil || !item.DispatchPending || item.ID != "first" {
		t.Fatalf("unknown claim treated fresh: %+v %v", item, err)
	}
	if _, err := store.Dispatch(location, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = NewInputQueueStore("").Snapshot(location)
	if err != nil || len(items) != 1 || items[0].ID != "second" {
		t.Fatalf("acceptance not committed: %+v %v", items, err)
	}
}

func TestQueueRecoveryCoalescesPendingCoordinatorWake(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	location := InputQueueLocation{Scope: protocol.InputQueueScopeDM, WorkspacePath: root, SessionKey: "agent:lead:ws:dm:recover"}
	item := protocol.InputQueueItem{Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: "execution", Content: "inspect current work"}
	first, err := store.EnqueueIdempotent(location, item, "startup:execution:1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.EnqueueIdempotent(location, item, "failed:attempt")
	if err != nil || !second.Duplicate || second.Item.ID != first.Item.ID || len(second.Items) != 1 {
		t.Fatalf("duplicate coordinator turn: %+v %v", second, err)
	}
	if _, err := store.Dispatch(location, first.Item.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := NewInputQueueStore(root).EnqueueIdempotent(location, item, "failed:attempt")
	if err != nil || !retry.Duplicate || len(retry.Items) != 0 {
		t.Fatalf("coalesced receipt lost: %+v %v", retry, err)
	}
	next, err := store.EnqueueIdempotent(location, item, "startup:execution:2")
	if err != nil || next.Duplicate || len(next.Items) != 1 {
		t.Fatalf("new recovery suppressed: %+v %v", next, err)
	}
}

func TestQueueDiscoverySkipsUnreadableJournalAndKeepsOthers(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	workspace := filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a")
	good := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: workspace, SessionKey: "agent:agent-a:ws:dm:good"}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(good, protocol.InputQueueItem{ID: "x", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(workspace, ".agents", "sessions", "bad-session")
	if err := os.MkdirAll(badDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.jsonl")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(badDir, "input_queue.jsonl")); err != nil {
		t.Fatal(err)
	}

	locations, err := store.LocationsForWorkspace("owner", workspace)

	if err == nil {
		t.Fatal("unreadable journal was not reported")
	}
	if len(locations) != 1 || locations[0].SessionKey != good.SessionKey {
		t.Fatalf("locations = %+v, want the readable queue still discovered", locations)
	}
}
