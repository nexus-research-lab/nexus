package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type historyBackfillFake struct {
	completed map[string]bool
}

func (f *historyBackfillFake) ListRuntimeHistoryBackfills(_ context.Context, after protocol.Execution, _ int) ([]protocol.Execution, error) {
	var result []protocol.Execution
	for _, id := range []string{"a", "b"} {
		if !f.completed[id] && id > after.ID {
			result = append(result, protocol.Execution{ID: id, OwnerUserID: "owner", SessionKey: "session"})
		}
	}
	return result, nil
}
func (f *historyBackfillFake) RuntimeGraphForMigration(context.Context, protocol.Execution) (protocol.ExecutionRuntimeGraph, error) {
	return protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{ID: "launch", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Agent", SubjectID: "launch-use"}}}, nil
}
func (f *historyBackfillFake) CompleteRuntimeHistoryBackfill(_ context.Context, e protocol.Execution, _ protocol.ExecutionRuntimeGraph) error {
	f.completed[e.ID] = true
	return nil
}

func TestWorkGraphHistoryRetriesFailuresAndReadsEachSessionOnce(t *testing.T) {
	repository := &historyBackfillFake{completed: map[string]bool{}}
	calls := 0
	unavailable := errors.New("transcript unavailable")
	source := func(ctx context.Context, owner, session string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
		calls++
		if authctx.OwnerUserID(ctx) != owner || session != "session" {
			t.Fatal("migration lost owner scope")
		}
		return nil, nil, unavailable
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); !errors.Is(err, unavailable) {
		t.Fatalf("lost retryable failure: %v", err)
	}
	if calls != 1 || len(repository.completed) != 0 {
		t.Fatal("failed history was marked done or session was repeatedly read")
	}
	unavailable = nil
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(repository.completed) != 2 {
		t.Fatal("retry did not finish the frozen backlog")
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("completed history was read again")
	}
}
