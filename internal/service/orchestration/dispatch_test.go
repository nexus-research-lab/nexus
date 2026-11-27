// dispatch_test.go 覆盖编排派发、取消、批量指派、评审派发与状态迁移测试。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type assignmentTargetAuthorizerFunc func(context.Context, AssignmentTargetRequest) error

func (f assignmentTargetAuthorizerFunc) AuthorizeAssignmentTarget(
	ctx context.Context,
	request AssignmentTargetRequest,
) error {
	return f(ctx, request)
}

func TestAuthorizeRoomRuntimeTargetRequiresManagedAssignmentAndExactBinding(t *testing.T) {
	snapshot := roomAssignmentSnapshot()
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-1",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		OwnerAgentID: "agent-worker",
		Strategy:     protocol.AssignmentStrategyRoomMember,
		Status:       protocol.WorkAssignmentStatusAssigned,
		Version:      1,
	}}
	snapshot.Dispatches = []protocol.ExecutionDispatch{{
		ID:            "dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		TargetAgentID: "agent-worker",
		Status:        protocol.ExecutionDispatchStatusClaimed,
		Version:       2,
	}}
	snapshot.Attempts = []protocol.WorkAttempt{{
		ID:              "attempt-1",
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          snapshot.Plan.ID,
		WorkItemID:      "work-1",
		SpecID:          "spec-1",
		AssignmentID:    "assignment-1",
		DispatchID:      "dispatch-1",
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: "agent-worker",
		Status:          protocol.WorkAttemptStatusPending,
		Version:         1,
	}}
	service := testService(&fakeRepository{snapshot: snapshot})
	actor := ActorContext{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		AgentID:        "agent-worker",
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
	}
	err := service.AuthorizeRoomRuntimeTarget(context.Background(), actor, nil)
	if err != nil {
		t.Fatalf("conversation-only raw wake rejected: %v", err)
	}
	unassigned := actor
	unassigned.AgentID = "agent-unassigned"
	if err := service.AuthorizeRoomRuntimeTarget(context.Background(), unassigned, nil); err != nil {
		t.Fatalf("unassigned conversational target rejected: %v", err)
	}
	coordinator := actor
	coordinator.AgentID = snapshot.Execution.CoordinatorAgentID
	if err := service.AuthorizeRoomRuntimeTarget(context.Background(), coordinator, nil); err != nil {
		t.Fatalf("coordinator wake rejected: %v", err)
	}
	ambiguousSnapshot := cloneExecutionSnapshot(snapshot)
	ambiguousSnapshot.Assignments = append(
		ambiguousSnapshot.Assignments,
		protocol.WorkAssignment{
			ID:           "assignment-2",
			ExecutionID:  snapshot.Execution.ID,
			PlanID:       snapshot.Plan.ID,
			WorkItemID:   "work-2",
			SpecID:       "spec-2",
			OwnerAgentID: "agent-worker",
			Strategy:     protocol.AssignmentStrategyRoomMember,
			Status:       protocol.WorkAssignmentStatusAssigned,
			Version:      1,
		},
	)
	ambiguousSnapshot.PlanItems = append(
		ambiguousSnapshot.PlanItems,
		protocol.ExecutionPlanItem{
			PlanID:      snapshot.Plan.ID,
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-2",
			SpecID:      "spec-2",
			Required:    true,
		},
	)
	ambiguousService := testService(&fakeRepository{snapshot: ambiguousSnapshot})
	err = ambiguousService.AuthorizeRoomRuntimeTarget(context.Background(), actor, nil)
	if err != nil {
		t.Fatalf("conversation wake depended on Assignment cardinality: %v", err)
	}
	cleanRawSnapshot := cloneExecutionSnapshot(snapshot)
	cleanRawSnapshot.Attempts = nil
	cleanRawSnapshot.Dispatches = nil
	cleanRawService := testService(&fakeRepository{snapshot: cleanRawSnapshot})
	if err := cleanRawService.AuthorizeRoomRuntimeTarget(
		context.Background(),
		actor,
		nil,
	); err != nil {
		t.Fatalf("unique idle Assignment raw wake rejected: %v", err)
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	if err := service.AuthorizeRoomRuntimeTarget(context.Background(), actor, binding); err != nil {
		t.Fatalf("exact structured binding rejected: %v", err)
	}
	terminalSnapshot := cloneExecutionSnapshot(snapshot)
	terminalSnapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	terminalService := testService(&fakeRepository{snapshot: terminalSnapshot})
	err = terminalService.AuthorizeRoomRuntimeTarget(context.Background(), actor, binding)
	var terminalErr *DomainError
	if !errors.As(err, &terminalErr) || terminalErr.Code != ErrorCodeExecutionTerminal {
		t.Fatalf("terminal structured wake error = %v, want %s", err, ErrorCodeExecutionTerminal)
	}
	err = terminalService.AuthorizeRoomRuntimeTarget(context.Background(), coordinator, nil)
	if err != nil {
		t.Fatalf("terminal background Execution blocked conversation: %v", err)
	}
	stale := *binding
	stale.AttemptID = "attempt-stale"
	if err := service.AuthorizeRoomRuntimeTarget(context.Background(), actor, &stale); err == nil {
		t.Fatal("stale Attempt binding was admitted")
	}
	stalePlan := *snapshot.Plan
	stalePlan.ID = "plan-2"
	snapshot.Plan = &stalePlan
	if err := service.AuthorizeRoomRuntimeTarget(context.Background(), actor, nil); err != nil {
		t.Fatalf("stale Plan blocked conversation transport: %v", err)
	}

	legacy := testService(&fakeRepository{})
	if err := legacy.AuthorizeRoomRuntimeTarget(context.Background(), actor, nil); err != nil {
		t.Fatalf("unmanaged legacy Room transport rejected: %v", err)
	}
}

func TestActivateRoomAttemptStartsPendingRootWithSlotIdentity(t *testing.T) {
	snapshot := dispatchDeliverySnapshot()
	snapshot.Dispatches = []protocol.ExecutionDispatch{{
		ID:            "dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusDelivered,
		Version:       2,
	}}
	var captured orchestrationstore.StartAttemptCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		startAttempt: func(
			_ context.Context,
			command orchestrationstore.StartAttemptCommand,
		) (*protocol.ExecutionSnapshot, error) {
			captured = command
			updated := cloneExecutionSnapshot(snapshot)
			updated.Execution.Version++
			updated.Assignments[0].Status = protocol.WorkAssignmentStatusActive
			updated.Assignments[0].Version++
			updated.Attempts[0] = command.Attempt
			updated.Attempts[0].Status = protocol.WorkAttemptStatusRunning
			updated.Attempts[0].Version++
			return updated, nil
		},
	}
	service := testService(repository)
	actor := ActorContext{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		AgentID:        "agent-worker",
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		RootRoundID:    "root-round-1",
		RuntimeRoundID: "runtime-round-1",
		AgentRoundID:   "agent-round-1",
	}
	binding := protocol.ExecutionWorkBinding{
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	if err := service.ActivateRoomAttempt(context.Background(), actor, RoomAttemptActivationInput{
		Binding:           binding,
		RuntimeSessionKey: "runtime-session-1",
		RoomSessionID:     "room-session-1",
	}); err != nil {
		t.Fatal(err)
	}
	if captured.ExpectedExecutionVersion != snapshot.Execution.Version ||
		captured.ExpectedAssignmentVersion != 1 ||
		captured.ExpectedAttemptVersion != 1 ||
		captured.Attempt.ID != "attempt-1" ||
		captured.Attempt.RuntimeSessionKey != "runtime-session-1" ||
		captured.Attempt.RoomSessionID != "room-session-1" ||
		captured.Attempt.RuntimeRoundID != "runtime-round-1" ||
		captured.Attempt.AgentRoundID != "agent-round-1" {
		t.Fatalf("activation command = %+v", captured)
	}
}

func roomAssignmentSnapshot() *protocol.ExecutionSnapshot {
	snapshot := executionSnapshot()
	snapshot.Execution.Version = 5
	snapshot.Execution.SessionKey = "room:group:conversation-1"
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
	snapshot.Execution.RoomID = "room-1"
	snapshot.Execution.ConversationID = "conversation-1"
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-1",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
		Version:     1,
	}
	snapshot.WorkItems = []protocol.WorkItem{{
		ID:          "work-1",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "W1",
		Kind:        protocol.WorkItemKindProduce,
	}}
	snapshot.WorkItemStates = []protocol.WorkItemState{{
		WorkItemID:    "work-1",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-1",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	}}
	snapshot.WorkItemSpecs = []protocol.WorkItemSpec{{
		ID:                 "spec-1",
		WorkItemID:         "work-1",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Research",
		Objective:          "Collect evidence",
		Deliverable:        "Evidence set",
		AcceptanceCriteria: []string{"sources cited"},
	}}
	snapshot.PlanItems = []protocol.ExecutionPlanItem{{
		PlanID:      "plan-1",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-1",
		SpecID:      "spec-1",
		Required:    true,
	}}
	snapshot.ReadyWorkItemIDs = []string{"work-1"}
	return snapshot
}

func dispatchDeliverySnapshot() *protocol.ExecutionSnapshot {
	snapshot := roomAssignmentSnapshot()
	snapshot.WorkItemSpecs[0].InputRefs = []string{
		"brief://assignment",
		"artifact://accepted-upstream",
	}
	snapshot.WorkItems = append(snapshot.WorkItems, protocol.WorkItem{
		ID:          "work-upstream",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "upstream",
		Kind:        protocol.WorkItemKindProduce,
	})
	snapshot.WorkItemStates = append(snapshot.WorkItemStates, protocol.WorkItemState{
		WorkItemID:    "work-upstream",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-upstream",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	})
	snapshot.WorkItemSpecs = append(snapshot.WorkItemSpecs, protocol.WorkItemSpec{
		ID:                 "spec-upstream",
		WorkItemID:         "work-upstream",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Upstream",
		Objective:          "Verify source evidence",
		Deliverable:        "Verified source evidence",
		AcceptanceCriteria: []string{"evidence verified"},
	})
	snapshot.PlanItems = append(snapshot.PlanItems, protocol.ExecutionPlanItem{
		PlanID:      snapshot.Plan.ID,
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-upstream",
		SpecID:      "spec-upstream",
		Required:    true,
		Position:    1,
	})
	snapshot.Dependencies = []protocol.ExecutionPlanDependency{{
		PlanID:              snapshot.Plan.ID,
		ExecutionID:         snapshot.Execution.ID,
		WorkItemID:          "work-1",
		DependsOnWorkItemID: "work-upstream",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot.OutputClaims = []protocol.ExecutionPlanOutputClaim{
		{
			PlanID:      snapshot.Plan.ID,
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-1",
			SpecID:      "spec-1",
			Scope:       "file:reports/evidence.md",
			Mode:        protocol.WorkOutputScopeExclusive,
		},
		{
			PlanID:      snapshot.Plan.ID,
			ExecutionID: snapshot.Execution.ID,
			WorkItemID:  "work-upstream",
			SpecID:      "spec-upstream",
			Scope:       "file:artifacts/upstream.json",
			Mode:        protocol.WorkOutputScopeExclusive,
		},
	}
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-1",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		OwnerAgentID: "agent-worker",
		Strategy:     protocol.AssignmentStrategyRoomMember,
		Status:       protocol.WorkAssignmentStatusAssigned,
		Version:      1,
	}}
	snapshot.Attempts = []protocol.WorkAttempt{{
		ID:              "attempt-1",
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          snapshot.Plan.ID,
		WorkItemID:      "work-1",
		SpecID:          "spec-1",
		AssignmentID:    "assignment-1",
		DispatchID:      "dispatch-1",
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: "agent-worker",
		Status:          protocol.WorkAttemptStatusPending,
		Version:         1,
	}}
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-upstream",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-upstream",
		SpecID:           "spec-upstream",
		AssignmentID:     "assignment-upstream",
		AttemptID:        "attempt-upstream",
		Sequence:         1,
		SubmitterAgentID: "agent-upstream",
		ResultSummary:    "Verified upstream evidence",
		ResultRefs:       []string{"artifact://upstream-result"},
		Evidence:         []string{"evidence://official"},
	}}
	snapshot.Acceptances = []protocol.WorkAcceptance{{
		ID:           "acceptance-upstream",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-upstream",
		SpecID:       "spec-upstream",
		AssignmentID: "assignment-upstream",
		SubmissionID: "submission-upstream",
		Decision:     protocol.WorkAcceptanceAccepted,
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "evidence verified",
			Passed:    true,
			Evidence:  []string{"evidence://official"},
		}},
	}}
	return snapshot
}

type cancellationOutboxFake struct {
	*fakeRepository
	item protocol.ExecutionCancellationDispatch
}

func (f *cancellationOutboxFake) ListAvailableCancellationDispatches(
	context.Context,
	int,
) ([]protocol.ExecutionCancellationDispatch, error) {
	if f.item.Status != protocol.ExecutionCancellationDispatchPending {
		return nil, nil
	}
	return []protocol.ExecutionCancellationDispatch{f.item}, nil
}

func (f *cancellationOutboxFake) ClaimCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	_ time.Duration,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.Status != protocol.ExecutionCancellationDispatchPending {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = protocol.ExecutionCancellationDispatchClaimed
	f.item.Version++
	f.item.DeliveryAttempts++
	f.item.LeaseOwner = owner
	item := f.item
	return &item, nil
}

func (f *cancellationOutboxFake) ResolveCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	status protocol.ExecutionCancellationDispatchStatus,
	outcome protocol.ExecutionCancellationOutcome,
	limitationCode string,
	receipt string,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.LeaseOwner != owner ||
		f.item.Status != protocol.ExecutionCancellationDispatchClaimed {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = status
	f.item.Outcome = outcome
	f.item.LimitationCode = limitationCode
	f.item.Receipt = receipt
	f.item.Version++
	f.item.LeaseOwner = ""
	item := f.item
	return &item, nil
}

func (f *cancellationOutboxFake) RetryCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	_ time.Time,
	cause string,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.LeaseOwner != owner ||
		f.item.Status != protocol.ExecutionCancellationDispatchClaimed {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = protocol.ExecutionCancellationDispatchPending
	f.item.LastError = cause
	f.item.Version++
	f.item.LeaseOwner = ""
	item := f.item
	return &item, nil
}

type cancellationConsumerFunc func(
	context.Context,
	ExecutionCancellationDelivery,
) (ExecutionCancellationReceipt, error)

func (f cancellationConsumerFunc) DeliverExecutionCancellation(
	ctx context.Context,
	delivery ExecutionCancellationDelivery,
) (ExecutionCancellationReceipt, error) {
	return f(ctx, delivery)
}

func TestDispatchPendingCancellationsRetriesThenDeliversExactBinding(t *testing.T) {
	repository := &cancellationOutboxFake{
		fakeRepository: &fakeRepository{},
		item: protocol.ExecutionCancellationDispatch{
			ID:                "cancel-1",
			ExecutionID:       "execution-old",
			PlanID:            "plan-old",
			WorkItemID:        "work-old",
			SpecID:            "spec-old",
			AssignmentID:      "assignment-old",
			AttemptID:         "attempt-child",
			RuntimeAttemptID:  "attempt-root",
			DispatchID:        "dispatch-old",
			ExecutorKind:      protocol.AttemptExecutorSubagent,
			TargetKind:        protocol.ExecutionCancellationTargetRuntimeRound,
			TargetAgentID:     "agent-worker",
			ScopeSessionKey:   "agent:worker:ws:dm:scope",
			RuntimeSessionKey: "agent:worker:ws:dm:runtime",
			RuntimeRoundID:    "round-old",
			Status:            protocol.ExecutionCancellationDispatchPending,
			Reason:            "execution superseded",
			Version:           1,
		},
	}
	service := NewService(repository)
	calls := 0
	var captured ExecutionCancellationDelivery
	service.SetExecutionCancellationConsumer(cancellationConsumerFunc(
		func(
			_ context.Context,
			delivery ExecutionCancellationDelivery,
		) (ExecutionCancellationReceipt, error) {
			calls++
			captured = delivery
			if calls == 1 {
				return ExecutionCancellationReceipt{}, errors.New(
					"runtime temporarily unavailable",
				)
			}
			return ExecutionCancellationReceipt{
				Outcome:        protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
				LimitationCode: "provider_interrupt_unsafe_shared_session",
				Detail:         "old local round cancelled",
			}, nil
		},
	))
	first, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-1",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Claimed != 1 || first.Retried != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchPending ||
		repository.item.LastError == "" {
		t.Fatalf("first drain = %+v item=%+v", first, repository.item)
	}
	second, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-2",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.Claimed != 1 || second.Delivered != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchDelivered ||
		repository.item.Outcome != protocol.ExecutionCancellationOutcomeLocalRoundCancelled ||
		repository.item.LimitationCode != "provider_interrupt_unsafe_shared_session" {
		t.Fatalf("second drain = %+v item=%+v", second, repository.item)
	}
	if captured.Binding.AttemptID != "attempt-child" ||
		captured.Binding.RuntimeAttemptID != "attempt-root" ||
		captured.Binding.RuntimeSessionKey != "agent:worker:ws:dm:runtime" ||
		captured.Binding.RuntimeRoundID != "round-old" ||
		captured.ExecutorKind != protocol.AttemptExecutorSubagent {
		t.Fatalf("captured delivery = %+v", captured)
	}
}

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

type executionReviewDispatchConsumerFunc func(
	context.Context,
	ExecutionReviewDispatchDelivery,
) (ExecutionReviewDispatchReceipt, error)

func (f executionReviewDispatchConsumerFunc) DeliverExecutionReviewDispatch(
	ctx context.Context,
	delivery ExecutionReviewDispatchDelivery,
) (ExecutionReviewDispatchReceipt, error) {
	return f(ctx, delivery)
}

type reviewDispatchRepositoryFake struct {
	*fakeRepository
	candidates []protocol.ExecutionReviewDispatch
	claim      func(
		string,
		int64,
		string,
		time.Duration,
	) (*protocol.ExecutionReviewDispatch, error)
	mark func(
		string,
		int64,
		string,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	retry func(
		string,
		int64,
		string,
		time.Time,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	cancel func(
		string,
		int64,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
}

func (f *reviewDispatchRepositoryFake) ListAvailableReviewDispatches(
	context.Context,
	int,
) ([]protocol.ExecutionReviewDispatch, error) {
	return append([]protocol.ExecutionReviewDispatch(nil), f.candidates...), nil
}

func (f *reviewDispatchRepositoryFake) ClaimReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	lease time.Duration,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.claim == nil {
		return nil, errors.New("unexpected ClaimReviewDispatch")
	}
	return f.claim(id, version, workerID, lease)
}

func (f *reviewDispatchRepositoryFake) MarkReviewDispatchDelivered(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	handoffID string,
	queueItemID string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.mark == nil {
		return nil, errors.New("unexpected MarkReviewDispatchDelivered")
	}
	return f.mark(id, version, workerID, handoffID, queueItemID)
}

func (f *reviewDispatchRepositoryFake) RetryReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.retry == nil {
		return nil, errors.New("unexpected RetryReviewDispatch")
	}
	return f.retry(id, version, workerID, retryAt, cause)
}

func (f *reviewDispatchRepositoryFake) CancelReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	reason string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.cancel == nil {
		return nil, errors.New("unexpected CancelReviewDispatch")
	}
	return f.cancel(id, version, workerID, reason)
}

func TestDispatchPendingReviewsCancelsTerminalReturnWithoutDelivery(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	claimed := dispatch
	claimed.Status = protocol.ExecutionReviewDispatchStatusClaimed
	claimed.Version = 2
	claimed.DeliveryAttempts = 1
	cancelled := false
	repository := &reviewDispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionReviewDispatch{dispatch},
		claim: func(
			string,
			int64,
			string,
			time.Duration,
		) (*protocol.ExecutionReviewDispatch, error) {
			return &claimed, nil
		},
		cancel: func(
			id string,
			version int64,
			workerID string,
			reason string,
		) (*protocol.ExecutionReviewDispatch, error) {
			cancelled = true
			if id != dispatch.ID || version != claimed.Version ||
				workerID != "review-worker" ||
				!strings.Contains(reason, "terminal") {
				t.Fatalf(
					"cancel identity = id=%q version=%d worker=%q reason=%q",
					id,
					version,
					workerID,
					reason,
				)
			}
			value := claimed
			value.Status = protocol.ExecutionReviewDispatchStatusCancelled
			return &value, nil
		},
	}
	service := testService(repository)
	service.SetExecutionReviewDispatchConsumer(
		executionReviewDispatchConsumerFunc(func(
			context.Context,
			ExecutionReviewDispatchDelivery,
		) (ExecutionReviewDispatchReceipt, error) {
			t.Fatal("terminal review return reached Room delivery")
			return ExecutionReviewDispatchReceipt{}, nil
		}),
	)
	result, err := service.DispatchPendingReviews(
		context.Background(),
		"review-worker",
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled || result.Cancelled != 1 || result.Delivered != 0 {
		t.Fatalf("terminal Dispatch result = %+v, cancelled=%t", result, cancelled)
	}
}

func reviewReturnSnapshot() (*protocol.ExecutionSnapshot, protocol.ExecutionReviewDispatch) {
	snapshot := roomAssignmentSnapshot()
	snapshot.Execution.CoordinatorAgentID = "agent-lead"
	snapshot.Execution.Status = protocol.ExecutionStatusActive
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:                "assignment-1",
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            snapshot.Plan.ID,
		WorkItemID:        "work-1",
		SpecID:            "spec-1",
		OwnerAgentID:      "agent-worker",
		AssignedByAgentID: "agent-lead",
		ReturnToAgentID:   "agent-lead",
		Strategy:          protocol.AssignmentStrategyRoomMember,
		Status:            protocol.WorkAssignmentStatusActive,
		Version:           3,
	}}
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed evidence",
	}}
	dispatch := protocol.ExecutionReviewDispatch{
		ID:            "review-dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		SubmissionID:  "submission-1",
		DedupeKey:     "review-return:submission-1",
		TargetAgentID: "agent-lead",
		Status:        protocol.ExecutionReviewDispatchStatusPending,
		Instruction:   "review evidence",
		Version:       1,
	}
	snapshot.ReviewDispatches = []protocol.ExecutionReviewDispatch{dispatch}
	return snapshot, dispatch
}

func TestServicePlanExecutionReplacesTransientWithCompleteSuccessor(t *testing.T) {
	old := executionSnapshot()
	old.Execution.CompletionCriteria = []string{"old criterion"}
	var captured orchestrationstore.ReplaceWithPlanCommand
	repository := &fakeRepository{
		snapshot: old,
		replaceWithPlan: func(
			_ context.Context,
			command orchestrationstore.ReplaceWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			captured = command
			return snapshotFromInitialPlan(command.Successor, command.Plan), nil
		},
	}
	service := testService(repository)
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:             old.Execution.ID,
		SnapshotRevision:        old.Execution.Version,
		CommandID:               "replace-objective",
		Objective:               "Deliver the replacement objective",
		CompletionCriteria:      []string{"replacement accepted"},
		ReplaceCurrentExecution: true,
		ReplacementReason:       "user explicitly changed the objective",
		Draft:                   validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		captured.ExecutionID != old.Execution.ID ||
		captured.Successor.ReplacesExecutionID != old.Execution.ID ||
		captured.Successor.Objective == old.Execution.Objective ||
		captured.Plan.Plan.ExecutionID != captured.Successor.ID ||
		result.ExecutionID != captured.Successor.ID {
		t.Fatalf("replacement result=%#v command=%#v", result, captured)
	}
}

func TestServicePlanExecutionRejectsGoalBoundReplacement(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-1"
	snapshot.Execution.GoalObjectiveRevision = 1
	snapshot.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	snapshot.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	service := testService(&fakeRepository{snapshot: snapshot})
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:             snapshot.Execution.ID,
		SnapshotRevision:        snapshot.Execution.Version,
		CommandID:               "replace-goal",
		Objective:               "different",
		CompletionCriteria:      []string{"accepted"},
		ReplaceCurrentExecution: true,
		ReplacementReason:       "user changed objective",
		Draft:                   validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReasonCode != ErrorCodeGoalRetargetRequired {
		t.Fatalf("Goal-bound replacement result = %#v", result)
	}
}

func TestServiceAbandonExecutionWritesOnlyOutsidePlanMode(t *testing.T) {
	for _, test := range []struct {
		name     string
		planMode bool
		outcome  MutationOutcome
		calls    int
	}{
		{name: "validate", planMode: true, outcome: MutationNoOp, calls: 0},
		{name: "apply", outcome: MutationApplied, calls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := executionSnapshot()
			calls := 0
			repository := &fakeRepository{
				snapshot: snapshot,
				abandon: func(
					_ context.Context,
					command orchestrationstore.AbandonCommand,
				) (*protocol.ExecutionSnapshot, error) {
					calls++
					updated := cloneExecutionSnapshot(snapshot)
					updated.Execution.Version++
					updated.Execution.Status = protocol.ExecutionStatusCancelled
					return updated, nil
				},
			}
			service := testService(repository)
			actor := coordinatorActor()
			actor.PlanMode = test.planMode
			result, err := service.AbandonExecution(context.Background(), actor, AbandonExecutionInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        "abandon",
				Reason:           "user stopped this objective",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != test.outcome || calls != test.calls {
				t.Fatalf("result=%#v calls=%d", result, calls)
			}
		})
	}
}

func TestServiceTerminalExecutionRejectsLateMutation(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	service := testService(&fakeRepository{snapshot: snapshot})
	result, err := service.AssignWork(context.Background(), coordinatorActor(), AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "late-assign",
		LogicalKey:       snapshot.WorkItems[0].LogicalKey,
		TargetAgentID:    "agent-worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReasonCode != ErrorCodeExecutionTerminal {
		t.Fatalf("terminal mutation result = %#v", result)
	}
}

func snapshotFromInitialPlan(
	execution protocol.Execution,
	command orchestrationstore.WritePlanCommand,
) *protocol.ExecutionSnapshot {
	execution.Version = 1
	plan := command.Plan
	plan.Version = 1
	result := &protocol.ExecutionSnapshot{
		Execution:    execution,
		Plan:         &plan,
		Dependencies: append([]protocol.ExecutionPlanDependency(nil), command.Dependencies...),
	}
	for _, item := range command.WorkItems {
		result.WorkItems = append(result.WorkItems, item.WorkItem)
		result.WorkItemSpecs = append(result.WorkItemSpecs, item.Spec)
		result.WorkItemStates = append(result.WorkItemStates, item.State)
		result.PlanItems = append(result.PlanItems, item.Item)
		for _, claim := range item.OutputClaims {
			claim.ExecutionID = execution.ID
			claim.PlanID = plan.ID
			claim.WorkItemID = item.WorkItem.ID
			claim.SpecID = item.Spec.ID
			result.OutputClaims = append(result.OutputClaims, claim)
		}
	}
	return result
}
