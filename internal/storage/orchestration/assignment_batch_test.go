package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAssignmentBatchAtomicRollbackAndDurableReplay(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := repository.Create(ctx, createTestCommand("batch")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.WritePlan(ctx, testPlanCommand("batch", 1, "batch", "", 1))
	if err != nil {
		t.Fatal(err)
	}
	batch := AssignBatchCommand{
		ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: snapshot.Execution.Version,
		RequestDigest: "exact-intent", Meta: testMeta("batch"),
	}
	for i := 1; i <= 2; i++ {
		suffix := fmt.Sprint(i)
		item := assignTestCommand(snapshot, "work-batch-"+suffix, "spec-batch-"+suffix, "batch-"+suffix, "agent-worker-"+suffix)
		item.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
		item.Dispatch = &protocol.ExecutionDispatch{
			ID: "dispatch-" + suffix, DedupeKey: "batch-dispatch-" + suffix, TargetAgentID: item.Assignment.OwnerAgentID,
			Kind: protocol.ExecutionDispatchRoomDirected, Status: protocol.ExecutionDispatchStatusPending,
			Instruction: "deliver evidence",
		}
		batch.Assignments = append(batch.Assignments, item)
	}
	assertUnchanged := func() {
		t.Helper()
		current, err := repository.GetSnapshot(ctx, batch.ExecutionID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Execution.Version != snapshot.Execution.Version || len(current.Assignments) != 0 || len(current.Attempts) != 0 || len(current.Dispatches) != 0 {
			t.Fatalf("failed batch left writes: %+v", current)
		}
		assertEventSequence(t, repository.db, batch.ExecutionID, 2)
	}
	invalid := batch
	invalid.Assignments = append([]AssignCommand(nil), batch.Assignments...)
	invalid.Assignments[1].Assignment.SpecID = "wrong-spec"
	if _, err := repository.AssignBatch(ctx, invalid); err == nil {
		t.Fatal("second invalid assignment committed")
	}
	assertUnchanged()
	duplicate := batch
	duplicate.Assignments = []AssignCommand{batch.Assignments[0], batch.Assignments[0]}
	if _, err := repository.AssignBatch(ctx, duplicate); !errors.Is(err, ErrInvariant) {
		t.Fatalf("duplicate work error = %v", err)
	}
	assertUnchanged()
	stale := batch
	stale.ExpectedExecutionVersion--
	if _, err := repository.AssignBatch(ctx, stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale batch error = %v", err)
	}
	assertUnchanged()
	for _, command := range []AssignBatchCommand{batch, batch} {
		committed, err := repository.AssignBatch(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		if committed.Execution.Version != snapshot.Execution.Version+1 || len(committed.Assignments) != 2 || len(committed.Attempts) != 2 || len(committed.Dispatches) != 2 {
			t.Fatalf("batch commit/replay = %+v", committed)
		}
		assertEventSequence(t, repository.db, batch.ExecutionID, 5)
	}
	changedIntent := batch
	changedIntent.RequestDigest = "different-intent"
	if _, err := repository.AssignBatch(ctx, changedIntent); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed request intent error = %v", err)
	}
	changedActor := batch
	changedActor.Meta.ActorID = "another-actor"
	if _, err := repository.AssignBatch(ctx, changedActor); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed actor error = %v", err)
	}
	assertEventSequence(t, repository.db, batch.ExecutionID, 5)
}
