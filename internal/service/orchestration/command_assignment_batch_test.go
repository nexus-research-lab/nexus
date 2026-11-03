package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type batchRepositoryFake struct {
	*fakeRepository
	receipt  *protocol.ExecutionEvent
	writes   int
	onCommit func(*protocol.ExecutionSnapshot, *protocol.ExecutionEvent)
}

func (r *batchRepositoryFake) FindAssignmentBatchReceipt(_ context.Context, _, commandID string) (*protocol.ExecutionEvent, error) {
	if r.receipt != nil && r.receipt.CommandID == commandID {
		return r.receipt, nil
	}
	return nil, nil
}

func (r *batchRepositoryFake) AssignBatch(_ context.Context, command orchestrationstore.AssignBatchCommand) (*protocol.ExecutionSnapshot, error) {
	r.writes++
	updated := cloneExecutionSnapshot(r.snapshot)
	updated.Execution.Version++
	for _, item := range command.Assignments {
		updated.Assignments = append(updated.Assignments, item.Assignment)
		updated.Attempts = append(updated.Attempts, *item.RootAttempt)
		if item.Dispatch != nil {
			updated.Dispatches = append(updated.Dispatches, *item.Dispatch)
		}
	}
	command.Meta.Payload["request_digest"] = command.RequestDigest
	r.receipt = &protocol.ExecutionEvent{
		ID: command.Meta.EventID, CommandID: command.Meta.CommandID, ActorID: command.Meta.ActorID,
		Type: protocol.ExecutionEventWorkAssigned, EntityType: protocol.ExecutionEntityExecution,
		Payload: command.Meta.Payload,
	}
	r.snapshot = updated
	if r.onCommit != nil {
		r.onCommit(updated, r.receipt)
	}
	return updated, nil
}

func TestAssignWorkBatchConcurrentWinnerSuppliesTheBinding(t *testing.T) {
	service, repository, actor, input := assignmentBatchFixture(t)
	repository.onCommit = func(snapshot *protocol.ExecutionSnapshot, receipt *protocol.ExecutionEvent) {
		// Model a same-request winner between the pre-read and repository mutation.
		receipt.ID = "concurrent-winner-event"
		snapshot.Assignments[2].ID = "winner-assignment"
		snapshot.Attempts[2].AssignmentID = "winner-assignment"
		snapshot.Attempts[2].ID = "winner-attempt"
		receipt.Payload["self_assignment_id"] = "winner-assignment"
		receipt.Payload["self_attempt_id"] = "winner-attempt"
	}
	result, err := service.AssignWorkBatch(context.Background(), actor, input)
	if err != nil || result.Outcome != MutationNoOp || len(result.Changed) != 0 || result.WorkBinding == nil || result.WorkBinding.Binding.AssignmentID != "winner-assignment" || result.WorkBinding.Binding.AttemptID != "winner-attempt" {
		t.Fatalf("concurrent receipt = %+v, err=%v", result, err)
	}
}

func TestAssignmentBatchContextOnlyAdvertisesCoordinatorLane(t *testing.T) {
	_, repository, actor, _ := assignmentBatchFixture(t)
	for _, test := range []struct {
		name    string
		options ExecutionContextOptions
		allowed bool
	}{
		{"coordinator", ExecutionContextOptions{Role: ExecutionActorCoordinator}, true},
		{"work", ExecutionContextOptions{Role: ExecutionActorCoordinator, WorkBound: true}, false},
		{"review", ExecutionContextOptions{Role: ExecutionActorCoordinator, ReviewBound: true}, false},
		{"observe", ExecutionContextOptions{Role: ExecutionActorCoordinator, ObserveOnly: true}, false},
		{"plan", ExecutionContextOptions{Role: ExecutionActorCoordinator, PlanMode: true}, false},
		{"member", ExecutionContextOptions{Role: ExecutionActorMember}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.options.ActorAgentID = actor.AgentID
			rendered := RenderExecutionContext(repository.snapshot, test.options)
			_, after, ok := strings.Cut(rendered, "<allowed_actions>")
			allowed, _, _ := strings.Cut(after, "</allowed_actions>")
			if !ok || strings.Contains(allowed, "<action>assign_work_batch</action>") != test.allowed {
				t.Fatalf("allowed actions = %s", allowed)
			}
		})
	}
}

func assignmentBatchFixture(t *testing.T) (*Service, *batchRepositoryFake, ActorContext, AssignWorkBatchInput) {
	t.Helper()
	snapshot := roomAssignmentSnapshot()
	for i := 2; i <= 3; i++ {
		workID, specID := fmt.Sprintf("work-%d", i), fmt.Sprintf("spec-%d", i)
		work, spec, planItem := snapshot.WorkItems[0], snapshot.WorkItemSpecs[0], snapshot.PlanItems[0]
		work.ID, work.LogicalKey = workID, workID
		spec.ID, spec.WorkItemID = specID, workID
		planItem.WorkItemID, planItem.SpecID = workID, specID
		snapshot.WorkItems = append(snapshot.WorkItems, work)
		snapshot.WorkItemSpecs = append(snapshot.WorkItemSpecs, spec)
		snapshot.PlanItems = append(snapshot.PlanItems, planItem)
		snapshot.ReadyWorkItemIDs = append(snapshot.ReadyWorkItemIDs, workID)
	}
	repository := &batchRepositoryFake{fakeRepository: &fakeRepository{snapshot: snapshot}}
	service := testService(repository)
	service.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(context.Context, AssignmentTargetRequest) error { return nil }))
	actor := coordinatorActor()
	actor.SessionKey, actor.ScopeKind = snapshot.Execution.SessionKey, protocol.ExecutionScopeRoom
	actor.RoomID, actor.ConversationID = snapshot.Execution.RoomID, snapshot.Execution.ConversationID
	actor.RuntimeRoundID = "batch-round"
	if err := service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}
	return service, repository, actor, AssignWorkBatchInput{
		ExecutionID: snapshot.Execution.ID, SnapshotRevision: snapshot.Execution.Version, CommandID: "batch-command",
		Assignments: []AssignmentBatchItem{
			{WorkItemID: "work-1", TargetAgentID: "agent-worker"},
			{WorkItemID: "work-2", TargetAgentID: "agent-other"},
			{WorkItemID: "work-3", TargetAgentID: actor.AgentID},
		},
	}
}

func TestAssignWorkBatchCommitsOnceAndReplaysExactSelfBinding(t *testing.T) {
	service, repository, actor, input := assignmentBatchFixture(t)
	input.CommandID = strings.Repeat("r", 128)
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)
	result, err := service.AssignWorkBatch(context.Background(), actor, input)
	if err != nil || result.Outcome != MutationApplied || repository.writes != 1 || len(result.Changed) != 8 || result.WorkBinding == nil {
		t.Fatalf("batch result = %+v, err=%v writes=%d", result, err, repository.writes)
	}
	if len(repository.receipt.CommandID) > 128 || len(sink.invalidations) != 1 {
		t.Fatalf("receipt identity or batch notification = %q / %+v", repository.receipt.CommandID, sink.invalidations)
	}
	if result.WorkBinding.Binding.WorkItemID != "work-3" || len(result.Snapshot.Assignments) != 3 || len(result.Snapshot.Dispatches) != 2 {
		t.Fatalf("batch state = %+v", result)
	}
	replay, err := service.AssignWorkBatch(context.Background(), actor, input) // same stale revision
	if err != nil || replay.Outcome != MutationNoOp || repository.writes != 1 || replay.WorkBinding == nil || *replay.WorkBinding.Binding != *result.WorkBinding.Binding {
		t.Fatalf("replay = %+v, err=%v", replay, err)
	}
	input.Assignments[0].TargetAgentID = "replacement-owner"
	reused, err := service.AssignWorkBatch(context.Background(), actor, input)
	if err != nil || reused.Outcome != MutationRejected || repository.writes != 1 {
		t.Fatalf("changed request = %+v, err=%v", reused, err)
	}
	input.Assignments[0].TargetAgentID = "agent-worker"
	// A takeover or resumed attempt must never regain authority from an old receipt.
	repository.snapshot.Assignments[2].ID = "replacement-assignment"
	repository.snapshot.Attempts[2].AssignmentID = "replacement-assignment"
	repository.snapshot.Attempts[2].ID = "replacement-attempt"
	replay, err = service.AssignWorkBatch(context.Background(), actor, input)
	if err != nil || replay.Outcome != MutationNoOp || replay.WorkBinding != nil || repository.writes != 1 {
		t.Fatalf("stale binding replay = %+v, err=%v", replay, err)
	}
}

func TestAssignWorkBatchRejectsWholeInvalidOrUnauthorizedBatch(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		mutate func(*Service, *batchRepositoryFake, *ActorContext, *AssignWorkBatchInput)
	}{
		{"duplicate", func(_ *Service, _ *batchRepositoryFake, _ *ActorContext, input *AssignWorkBatchInput) {
			input.Assignments[1].WorkItemID = "work-1"
		}},
		{"self-first", func(_ *Service, _ *batchRepositoryFake, actor *ActorContext, input *AssignWorkBatchInput) {
			input.Assignments[0].TargetAgentID = " " + actor.AgentID + " "
		}},
		{"not-ready", func(_ *Service, r *batchRepositoryFake, _ *ActorContext, _ *AssignWorkBatchInput) {
			r.snapshot.ReadyWorkItemIDs = []string{"work-1"}
		}},
		{"plan-mode", func(_ *Service, _ *batchRepositoryFake, actor *ActorContext, _ *AssignWorkBatchInput) {
			actor.PlanMode = true
		}},
		{"wrong-owner", func(_ *Service, _ *batchRepositoryFake, actor *ActorContext, _ *AssignWorkBatchInput) {
			actor.AgentID = "agent-worker"
		}},
		{"unentered-room", func(_ *Service, _ *batchRepositoryFake, actor *ActorContext, _ *AssignWorkBatchInput) {
			actor.RuntimeRoundID = "unentered-round"
		}},
		{"stale-revision", func(_ *Service, _ *batchRepositoryFake, _ *ActorContext, input *AssignWorkBatchInput) {
			input.SnapshotRevision--
		}},
		{"target-denied", func(s *Service, _ *batchRepositoryFake, _ *ActorContext, _ *AssignWorkBatchInput) {
			s.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(_ context.Context, request AssignmentTargetRequest) error {
				if request.TargetAgentID == "agent-other" {
					return fmt.Errorf("target denied")
				}
				return nil
			}))
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service, repository, actor, input := assignmentBatchFixture(t)
			sink := &capturingExecutionInvalidationSink{}
			service.SetExecutionInvalidationSink(sink)
			scenario.mutate(service, repository, &actor, &input)
			result, err := service.AssignWorkBatch(context.Background(), actor, input)
			if err != nil || result.Outcome != MutationRejected || repository.writes != 0 || result.WorkBinding != nil || len(sink.invalidations) != 0 {
				t.Fatalf("invalid batch = %+v, err=%v writes=%d", result, err, repository.writes)
			}
		})
	}
}
