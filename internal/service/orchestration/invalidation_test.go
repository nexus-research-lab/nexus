package orchestration

import (
	"context"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestEnsurePublishesAfterDurableCreate(t *testing.T) {
	repository := &fakeRepository{
		create: func(
			_ context.Context,
			command orchestrationstore.CreateCommand,
		) (*protocol.ExecutionSnapshot, error) {
			execution := command.Execution
			execution.Version = 1
			return &protocol.ExecutionSnapshot{Execution: execution}, nil
		},
	}
	service := testService(repository)
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)

	result, err := service.Ensure(
		context.Background(),
		coordinatorActor(),
		EnsureInput{
			CommandID:          "ensure-invalidation",
			Objective:          "Ship the graph",
			CompletionCriteria: []string{"visible after commit"},
		},
	)
	if err != nil || result.Outcome != MutationApplied {
		t.Fatalf("Ensure result=%#v err=%v", result, err)
	}
	if len(sink.invalidations) != 1 ||
		sink.invalidations[0].ExecutionID != result.ExecutionID ||
		sink.invalidations[0].Version != 1 {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
}

func TestPlanMaterializationPublishesManagedGraphInvalidation(t *testing.T) {
	repository := &fakeRepository{
		createWithPlan: func(
			_ context.Context,
			command orchestrationstore.CreateWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			return snapshotFromInitialPlan(command.Execution, command.Plan), nil
		},
	}
	service := testService(repository)
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)

	result, err := service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		PlanExecutionInput{
			CommandID:          "materialize-invalidation",
			Objective:          "Ship the graph",
			CompletionCriteria: []string{"the graph is accepted"},
			Draft:              validPlanDraft(),
		},
	)
	if err != nil || result.Outcome != MutationApplied || result.Snapshot.Plan == nil {
		t.Fatalf("PlanExecution result=%#v err=%v", result, err)
	}
	if len(sink.invalidations) != 1 ||
		sink.invalidations[0].ExecutionID != result.ExecutionID ||
		sink.invalidations[0].Version != result.Snapshot.Execution.Version {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
}

func TestTerminalMutationPublishesFinalSnapshotInvalidation(t *testing.T) {
	current := executionSnapshot()
	repository := &fakeRepository{
		snapshot: current,
		abandon: func(
			_ context.Context,
			_ orchestrationstore.AbandonCommand,
		) (*protocol.ExecutionSnapshot, error) {
			terminal := cloneExecutionSnapshot(current)
			terminal.Execution.Status = protocol.ExecutionStatusCancelled
			terminal.Execution.Version++
			return terminal, nil
		},
	}
	service := testService(repository)
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)

	result, err := service.AbandonExecution(
		context.Background(),
		coordinatorActor(),
		AbandonExecutionInput{
			ExecutionID:      current.Execution.ID,
			SnapshotRevision: current.Execution.Version,
			CommandID:        "terminal-invalidation",
			Reason:           "user cancelled the graph",
		},
	)
	if err != nil || result.Outcome != MutationApplied ||
		result.Snapshot.Execution.Status != protocol.ExecutionStatusCancelled {
		t.Fatalf("AbandonExecution result=%#v err=%v", result, err)
	}
	if len(sink.invalidations) != 1 ||
		sink.invalidations[0].Version != result.Snapshot.Execution.Version {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
}

func TestAppliedMutationPublishesExecutionInvalidation(t *testing.T) {
	sink := &capturingExecutionInvalidationSink{}
	service := NewService(&fakeRepository{})
	service.SetExecutionInvalidationSink(sink)
	service.invalidateMutationResult(context.Background(), AppliedResult(
		&protocol.ExecutionSnapshot{Execution: protocol.Execution{
			ID:          "execution-1",
			OwnerUserID: "owner-a",
			SessionKey:  "session-1",
			Version:     9,
		}},
		[]string{"execution:execution-1"},
		nil,
	), nil)

	if len(sink.invalidations) != 1 {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
	got := sink.invalidations[0]
	if got.OwnerUserID != "owner-a" || got.SessionKey != "session-1" ||
		got.ExecutionID != "execution-1" || got.Version != 9 {
		t.Fatalf("invalidation = %#v", got)
	}
}

func TestRejectedMutationDoesNotPublishExecutionInvalidation(t *testing.T) {
	sink := &capturingExecutionInvalidationSink{}
	service := NewService(&fakeRepository{})
	service.SetExecutionInvalidationSink(sink)
	service.invalidateMutationResult(context.Background(), RejectedResult(
		&protocol.ExecutionSnapshot{Execution: protocol.Execution{
			ID:          "execution-1",
			OwnerUserID: "owner-a",
			SessionKey:  "session-1",
		}},
		domainError(ErrorCodeInvalidInput, "no"),
		nil,
	), nil)
	if len(sink.invalidations) != 0 {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
}

func TestIdempotentReplayPublishesExecutionInvalidation(t *testing.T) {
	sink := &capturingExecutionInvalidationSink{}
	service := NewService(&fakeRepository{})
	service.SetExecutionInvalidationSink(sink)
	service.invalidateMutationResult(context.Background(), NoOpResult(
		&protocol.ExecutionSnapshot{Execution: protocol.Execution{
			ID:          "execution-1",
			OwnerUserID: "owner-a",
			SessionKey:  "session-1",
			Version:     9,
		}},
		"command already applied",
	), nil)

	if len(sink.invalidations) != 1 {
		t.Fatalf("invalidations = %#v", sink.invalidations)
	}
}

func TestExecutionInvalidationDropsIncompleteOwnerSessionScope(t *testing.T) {
	sink := &capturingExecutionInvalidationSink{}
	service := NewService(&fakeRepository{})
	service.SetExecutionInvalidationSink(sink)

	service.invalidateExecution(context.Background(), ExecutionInvalidation{
		SessionKey: "session-1",
	})
	service.invalidateExecution(context.Background(), ExecutionInvalidation{
		OwnerUserID: "owner-a",
	})
	if len(sink.invalidations) != 0 {
		t.Fatalf("incomplete owner/session scope escaped: %#v", sink.invalidations)
	}
}

func TestRuntimeLifecyclePublishesOneInvalidationPerObservedMutation(t *testing.T) {
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	}
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)
	actor := ActorContext{
		OwnerUserID:    "owner-a",
		SessionKey:     "session-1",
		AgentID:        "agent-1",
		RootRoundID:    "round-1",
		RuntimeRoundID: "round-1",
		AgentRoundID:   "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant",
		"uuid": "assistant-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type":  "tool_use",
				"id":    "tool-1",
				"name":  "search",
				"input": map[string]any{"query": "workgraph"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(sink.invalidations) != 1 {
		t.Fatalf("ObserveRuntimeMessage invalidations = %#v, want one", sink.invalidations)
	}
	if err = service.FinishRuntimeRound(
		context.Background(),
		actor,
		"completed",
		"",
	); err != nil {
		t.Fatal(err)
	}
	if len(sink.invalidations) != 2 {
		t.Fatalf("runtime mutation invalidations = %#v, want two", sink.invalidations)
	}
}

type capturingExecutionInvalidationSink struct {
	invalidations []ExecutionInvalidation
}

func (s *capturingExecutionInvalidationSink) InvalidateExecution(
	_ context.Context,
	invalidation ExecutionInvalidation,
) {
	s.invalidations = append(s.invalidations, invalidation)
}
