package orchestration

import (
	"context"
	"errors"
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

type executionDispatchConsumerFunc func(
	context.Context,
	ExecutionDispatchDelivery,
) (ExecutionDispatchReceipt, error)

func (f executionDispatchConsumerFunc) DeliverExecutionDispatch(
	ctx context.Context,
	delivery ExecutionDispatchDelivery,
) (ExecutionDispatchReceipt, error) {
	return f(ctx, delivery)
}

func TestExecutionDispatchWorkContractFailsClosedForHistoricalProjectionOverflow(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	planItem := snapshot.PlanItems[0]
	planItem.ExecutionID = snapshot.Execution.ID
	snapshot.PlanItems[0] = planItem
	for index := range snapshot.WorkItemSpecs {
		if snapshot.WorkItemSpecs[index].ID == planItem.SpecID {
			snapshot.WorkItemSpecs[index].InputRefs = make(
				[]string,
				protocol.ExecutionProjectionCollectionLimit+1,
			)
			break
		}
	}
	_, err := executionDispatchWorkContract(
		&snapshot,
		snapshot.Plan.ID,
		planItem.WorkItemID,
		planItem.SpecID,
	)
	if !errors.Is(err, protocol.ErrExecutionProjectionLimitExceeded) {
		t.Fatalf("error = %v", err)
	}
}

type dispatchRepositoryFake struct {
	*fakeRepository
	candidates []protocol.ExecutionDispatch
	claim      func(string, int64, string, time.Duration) (*protocol.ExecutionDispatch, error)
	mark       func(string, int64, string, string, string) (*protocol.ExecutionDispatch, error)
	retry      func(string, int64, string, time.Time, string) (*protocol.ExecutionDispatch, error)
	cancel     func(string, int64, string, string) (*protocol.ExecutionDispatch, error)
}

func (f *dispatchRepositoryFake) ListAvailableRoomDispatches(
	context.Context,
	int,
) ([]protocol.ExecutionDispatch, error) {
	return append([]protocol.ExecutionDispatch(nil), f.candidates...), nil
}

func (f *dispatchRepositoryFake) ClaimDispatch(
	_ context.Context,
	dispatchID string,
	expectedVersion int64,
	workerID string,
	leaseDuration time.Duration,
) (*protocol.ExecutionDispatch, error) {
	if f.claim == nil {
		return nil, errors.New("unexpected ClaimDispatch")
	}
	return f.claim(dispatchID, expectedVersion, workerID, leaseDuration)
}

func (f *dispatchRepositoryFake) MarkDispatchDelivered(
	_ context.Context,
	dispatchID string,
	expectedVersion int64,
	workerID string,
	handoffID string,
	queueItemID string,
) (*protocol.ExecutionDispatch, error) {
	if f.mark == nil {
		return nil, errors.New("unexpected MarkDispatchDelivered")
	}
	return f.mark(dispatchID, expectedVersion, workerID, handoffID, queueItemID)
}

func (f *dispatchRepositoryFake) RetryDispatch(
	_ context.Context,
	dispatchID string,
	expectedVersion int64,
	workerID string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionDispatch, error) {
	if f.retry == nil {
		return nil, errors.New("unexpected RetryDispatch")
	}
	return f.retry(dispatchID, expectedVersion, workerID, retryAt, cause)
}

func (f *dispatchRepositoryFake) CancelDispatch(
	_ context.Context,
	dispatchID string,
	expectedVersion int64,
	workerID string,
	cause string,
) (*protocol.ExecutionDispatch, error) {
	if f.cancel == nil {
		return nil, errors.New("unexpected CancelDispatch")
	}
	return f.cancel(dispatchID, expectedVersion, workerID, cause)
}

func TestAssignWorkRejectsInvalidTargetBeforePersistence(t *testing.T) {
	roomSnapshot := roomAssignmentSnapshot()
	tests := []struct {
		name       string
		snapshot   *protocol.ExecutionSnapshot
		target     string
		strategy   protocol.AssignmentStrategy
		dispatch   protocol.ExecutionDispatchKind
		authorizer AssignmentTargetAuthorizer
	}{
		{
			name:     "self cannot target another Agent",
			snapshot: roomSnapshot,
			target:   "agent-worker",
			strategy: protocol.AssignmentStrategySelf,
		},
		{
			name:     "self cannot request Room dispatch",
			snapshot: roomSnapshot,
			target:   "agent-lead",
			strategy: protocol.AssignmentStrategySelf,
			dispatch: protocol.ExecutionDispatchRoomDirected,
		},
		{
			name:     "Room member cannot alias self",
			snapshot: roomSnapshot,
			target:   "agent-lead",
			strategy: protocol.AssignmentStrategyRoomMember,
		},
		{
			name:     "non member is rejected by Room preflight",
			snapshot: roomSnapshot,
			target:   "agent-outsider",
			strategy: protocol.AssignmentStrategyRoomMember,
			authorizer: assignmentTargetAuthorizerFunc(func(
				context.Context,
				AssignmentTargetRequest,
			) error {
				return errors.New("not a Room member")
			}),
		},
		{
			name: "DM cannot create Room member assignment",
			snapshot: func() *protocol.ExecutionSnapshot {
				value := roomAssignmentSnapshot()
				value.Execution.ScopeKind = protocol.ExecutionScopeDM
				value.Execution.SessionKey = "agent:lead:workspace:dm:conversation-1"
				value.Execution.RoomID = ""
				value.Execution.ConversationID = ""
				return value
			}(),
			target:   "agent-worker",
			strategy: protocol.AssignmentStrategyRoomMember,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := testService(&fakeRepository{snapshot: test.snapshot})
			service.SetAssignmentTargetAuthorizer(test.authorizer)
			actor := coordinatorActor()
			actor.SessionKey = test.snapshot.Execution.SessionKey
			actor.ScopeKind = test.snapshot.Execution.ScopeKind
			actor.RoomID = test.snapshot.Execution.RoomID
			actor.ConversationID = test.snapshot.Execution.ConversationID
			actor.RuntimeRoundID = "assign-invalid-round"
			if actor.ScopeKind == protocol.ExecutionScopeRoom {
				if err := service.mintRuntimeCoordination(actor, test.snapshot.Execution.ID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
				ExecutionID:      test.snapshot.Execution.ID,
				SnapshotRevision: test.snapshot.Execution.Version,
				CommandID:        "assign-invalid-target",
				WorkItemID:       "work-1",
				TargetAgentID:    test.target,
				Strategy:         test.strategy,
				DispatchKind:     test.dispatch,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationRejected ||
				result.ReasonCode != ErrorCodeAssignmentTargetInvalid {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestAssignWorkAllowsRoomCoordinatorSelfAssignmentAndReview(t *testing.T) {
	snapshot := roomAssignmentSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	repository.assign = func(_ context.Context, command orchestrationstore.AssignCommand) (*protocol.ExecutionSnapshot, error) {
		if command.Assignment.OwnerAgentID != "agent-lead" ||
			command.Assignment.ReturnToAgentID != "agent-lead" ||
			command.Assignment.Strategy != protocol.AssignmentStrategySelf ||
			command.Dispatch != nil {
			t.Fatalf("Room coordinator Assignment = %+v, dispatch=%+v", command.Assignment, command.Dispatch)
		}
		result := cloneExecutionSnapshot(snapshot)
		result.Execution.Version++
		result.Assignments = append(result.Assignments, command.Assignment)
		result.Attempts = append(result.Attempts, *command.RootAttempt)
		return result, nil
	}
	service := testService(repository)
	actor := coordinatorActor()
	actor.SessionKey = snapshot.Execution.SessionKey
	actor.ScopeKind = protocol.ExecutionScopeRoom
	actor.RoomID = snapshot.Execution.RoomID
	actor.ConversationID = snapshot.Execution.ConversationID
	actor.RuntimeRoundID = "assign-room-self-round"
	if err := service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}

	result, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "assign-room-self",
		WorkItemID:       "work-1",
		TargetAgentID:    actor.AgentID,
		Strategy:         protocol.AssignmentStrategySelf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied {
		t.Fatalf("result = %+v", result)
	}
}

func TestAssignWorkAllowsRoomCoordinatorWorkWithIndependentReviewer(t *testing.T) {
	snapshot := roomAssignmentSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	repository.assign = func(_ context.Context, command orchestrationstore.AssignCommand) (*protocol.ExecutionSnapshot, error) {
		if command.Assignment.OwnerAgentID != "agent-lead" ||
			command.Assignment.ReturnToAgentID != "agent-reviewer" ||
			command.Assignment.Strategy != protocol.AssignmentStrategySelf ||
			command.Dispatch != nil {
			t.Fatalf("Room coordinator Assignment = %+v, dispatch=%+v", command.Assignment, command.Dispatch)
		}
		result := cloneExecutionSnapshot(snapshot)
		result.Execution.Version++
		result.Assignments = append(result.Assignments, command.Assignment)
		result.Attempts = append(result.Attempts, *command.RootAttempt)
		return result, nil
	}
	service := testService(repository)
	service.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(
		_ context.Context,
		request AssignmentTargetRequest,
	) error {
		if request.TargetAgentID != "agent-reviewer" {
			t.Fatalf("independent reviewer preflight = %+v", request)
		}
		return nil
	}))
	actor := coordinatorActor()
	actor.SessionKey = snapshot.Execution.SessionKey
	actor.ScopeKind = protocol.ExecutionScopeRoom
	actor.RoomID = snapshot.Execution.RoomID
	actor.ConversationID = snapshot.Execution.ConversationID
	actor.RuntimeRoundID = "assign-room-independent-review-round"
	if err := service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}

	result, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "assign-room-lead-work",
		WorkItemID:       "work-1",
		TargetAgentID:    actor.AgentID,
		ReturnToAgentID:  "agent-reviewer",
		Strategy:         protocol.AssignmentStrategySelf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied {
		t.Fatalf("result = %+v", result)
	}
}

func TestAssignWorkAllowsRoomMemberToRemainItsSelectedReviewer(t *testing.T) {
	snapshot := roomAssignmentSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	repository.assign = func(_ context.Context, command orchestrationstore.AssignCommand) (*protocol.ExecutionSnapshot, error) {
		if command.Assignment.OwnerAgentID != "agent-worker" ||
			command.Assignment.ReturnToAgentID != "agent-worker" ||
			command.Assignment.Strategy != protocol.AssignmentStrategyRoomMember ||
			command.Dispatch == nil {
			t.Fatalf("self-reviewing member Assignment = %+v, dispatch=%+v", command.Assignment, command.Dispatch)
		}
		result := cloneExecutionSnapshot(snapshot)
		result.Execution.Version++
		result.Assignments = append(result.Assignments, command.Assignment)
		result.Dispatches = append(result.Dispatches, *command.Dispatch)
		result.Attempts = append(result.Attempts, *command.RootAttempt)
		return result, nil
	}
	authorizedTargets := make([]string, 0, 1)
	service := testService(repository)
	service.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(
		_ context.Context,
		request AssignmentTargetRequest,
	) error {
		authorizedTargets = append(authorizedTargets, request.TargetAgentID)
		return nil
	}))
	actor := coordinatorActor()
	actor.SessionKey = snapshot.Execution.SessionKey
	actor.ScopeKind = protocol.ExecutionScopeRoom
	actor.RoomID = snapshot.Execution.RoomID
	actor.ConversationID = snapshot.Execution.ConversationID
	actor.RuntimeRoundID = "assign-room-member-self-review-round"
	if err := service.mintRuntimeCoordination(actor, snapshot.Execution.ID); err != nil {
		t.Fatal(err)
	}

	result, err := service.AssignWork(context.Background(), actor, AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "assign-member-self-review",
		WorkItemID:       "work-1",
		TargetAgentID:    "agent-worker",
		ReturnToAgentID:  "agent-worker",
		Strategy:         protocol.AssignmentStrategyRoomMember,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied {
		t.Fatalf("result = %+v", result)
	}
	if len(authorizedTargets) != 1 || authorizedTargets[0] != "agent-worker" {
		t.Fatalf("authorized targets = %#v, want the member checked once", authorizedTargets)
	}
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

func TestAuthorizeRoomRuntimeTargetConversationIgnoresWorkEvidence(t *testing.T) {
	base := roomAssignmentSnapshot()
	base.Assignments = []protocol.WorkAssignment{{
		ID:           "assignment-1",
		ExecutionID:  base.Execution.ID,
		PlanID:       base.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		OwnerAgentID: "agent-worker",
		Strategy:     protocol.AssignmentStrategyRoomMember,
		Status:       protocol.WorkAssignmentStatusActive,
		Version:      1,
	}}
	actor := ActorContext{
		OwnerUserID:    base.Execution.OwnerUserID,
		SessionKey:     base.Execution.SessionKey,
		AgentID:        "agent-worker",
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         base.Execution.RoomID,
		ConversationID: base.Execution.ConversationID,
	}
	for _, testCase := range []struct {
		name   string
		mutate func(*protocol.ExecutionSnapshot)
	}{
		{
			name: "running attempt",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.Attempts = []protocol.WorkAttempt{{
					ID:           "attempt-1",
					ExecutionID:  snapshot.Execution.ID,
					PlanID:       snapshot.Plan.ID,
					WorkItemID:   "work-1",
					SpecID:       "spec-1",
					AssignmentID: "assignment-1",
					Status:       protocol.WorkAttemptStatusRunning,
				}}
			},
		},
		{
			name: "delivered dispatch",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.Dispatches = []protocol.ExecutionDispatch{{
					ID:           "dispatch-1",
					ExecutionID:  snapshot.Execution.ID,
					PlanID:       snapshot.Plan.ID,
					WorkItemID:   "work-1",
					SpecID:       "spec-1",
					AssignmentID: "assignment-1",
					Status:       protocol.ExecutionDispatchStatusDelivered,
				}}
			},
		},
		{
			name: "unreviewed submission",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.Submissions = []protocol.WorkSubmission{{
					ID:           "submission-1",
					ExecutionID:  snapshot.Execution.ID,
					PlanID:       snapshot.Plan.ID,
					WorkItemID:   "work-1",
					SpecID:       "spec-1",
					AssignmentID: "assignment-1",
				}}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot := cloneExecutionSnapshot(base)
			testCase.mutate(snapshot)
			err := testService(&fakeRepository{snapshot: snapshot}).
				AuthorizeRoomRuntimeTarget(context.Background(), actor, nil)
			if err != nil {
				t.Fatalf("conversation wake was coupled to work evidence: %v", err)
			}
		})
	}

	coordinator := actor
	coordinator.AgentID = base.Execution.CoordinatorAgentID
	duplicate := cloneExecutionSnapshot(base)
	duplicate.Attempts = []protocol.WorkAttempt{{
		ID:           "attempt-1",
		AssignmentID: "assignment-1",
		Status:       protocol.WorkAttemptStatusRunning,
	}}
	if err := testService(&fakeRepository{snapshot: duplicate}).
		AuthorizeRoomRuntimeTarget(context.Background(), coordinator, nil); err != nil {
		t.Fatalf("coordinator unbound wake rejected: %v", err)
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

func TestDispatchPendingDeliversCompleteBindingAndPersistsReceipt(t *testing.T) {
	snapshot := dispatchDeliverySnapshot()
	candidate := protocol.ExecutionDispatch{
		ID:               "dispatch-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		TargetAgentID:    "agent-worker",
		Kind:             protocol.ExecutionDispatchRoomDirected,
		Status:           protocol.ExecutionDispatchStatusPending,
		DedupeKey:        "dispatch:work-1:agent-worker",
		Instruction:      "Deliver the evidence set.",
		Version:          4,
		DeliveryAttempts: 0,
	}
	repository := &dispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionDispatch{candidate},
	}
	repository.claim = func(
		dispatchID string,
		expectedVersion int64,
		workerID string,
		leaseDuration time.Duration,
	) (*protocol.ExecutionDispatch, error) {
		if dispatchID != candidate.ID || expectedVersion != candidate.Version ||
			workerID != "worker-1" || leaseDuration <= 0 {
			t.Fatalf("claim = %q/%d/%q/%s", dispatchID, expectedVersion, workerID, leaseDuration)
		}
		claimed := candidate
		claimed.Status = protocol.ExecutionDispatchStatusClaimed
		claimed.Version++
		claimed.DeliveryAttempts++
		return &claimed, nil
	}
	repository.mark = func(
		dispatchID string,
		expectedVersion int64,
		workerID string,
		handoffID string,
		queueItemID string,
	) (*protocol.ExecutionDispatch, error) {
		if dispatchID != candidate.ID || expectedVersion != 5 || workerID != "worker-1" ||
			handoffID != "execution_dispatch_dispatch-1" || queueItemID != "queue-1" {
			t.Fatalf(
				"mark = %q/%d/%q/%q/%q",
				dispatchID,
				expectedVersion,
				workerID,
				handoffID,
				queueItemID,
			)
		}
		delivered := candidate
		delivered.Status = protocol.ExecutionDispatchStatusDelivered
		delivered.Version = 6
		return &delivered, nil
	}
	var delivered ExecutionDispatchDelivery
	service := NewService(repository)
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		_ context.Context,
		delivery ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		delivered = delivery
		return ExecutionDispatchReceipt{
			HandoffID:   "execution_dispatch_dispatch-1",
			QueueItemID: "queue-1",
		}, nil
	}))

	result, err := service.DispatchPending(context.Background(), "worker-1", 8)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Delivered != 1 || result.Retried != 0 {
		t.Fatalf("DispatchRunResult = %+v", result)
	}
	wantBinding := protocol.ExecutionWorkBinding{
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       snapshot.Plan.ID,
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	if delivered.Binding != wantBinding ||
		delivered.SourceAgentID != snapshot.Execution.CoordinatorAgentID ||
		delivered.TargetAgentID != "agent-worker" ||
		delivered.DispatchDedupeKey != candidate.DedupeKey {
		t.Fatalf("delivery = %+v, want binding %+v", delivered, wantBinding)
	}
	if len(delivered.WorkContract.InputRefs) != 2 ||
		delivered.WorkContract.InputRefs[0] != "artifact://accepted-upstream" ||
		len(delivered.WorkContract.OutputScopes) != 1 ||
		delivered.WorkContract.OutputScopes[0].Scope != "file:reports/evidence.md" ||
		len(delivered.WorkContract.AcceptedDependencies) != 1 {
		t.Fatalf("delivery work contract = %+v", delivered.WorkContract)
	}
	dependency := delivered.WorkContract.AcceptedDependencies[0]
	if dependency.WorkItemID != "work-upstream" ||
		dependency.LogicalKey != "upstream" ||
		dependency.Kind != protocol.WorkDependencyHard ||
		dependency.SubmissionID != "submission-upstream" ||
		dependency.ResultSummary != "Verified upstream evidence" ||
		len(dependency.ResultRefs) != 1 ||
		dependency.ResultRefs[0] != "artifact://upstream-result" ||
		dependency.AcceptanceID != "acceptance-upstream" {
		t.Fatalf("accepted dependency contract = %+v", dependency)
	}
}

func TestDispatchPendingSchedulesRetryAfterDeliveryFailure(t *testing.T) {
	snapshot := dispatchDeliverySnapshot()
	candidate := protocol.ExecutionDispatch{
		ID:               "dispatch-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		TargetAgentID:    "agent-worker",
		Kind:             protocol.ExecutionDispatchRoomDirected,
		Status:           protocol.ExecutionDispatchStatusPending,
		Instruction:      "Deliver the evidence set.",
		Version:          7,
		DeliveryAttempts: 0,
	}
	fixedNow := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	repository := &dispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionDispatch{candidate},
	}
	repository.claim = func(
		_ string,
		_ int64,
		_ string,
		_ time.Duration,
	) (*protocol.ExecutionDispatch, error) {
		claimed := candidate
		claimed.Status = protocol.ExecutionDispatchStatusClaimed
		claimed.Version++
		claimed.DeliveryAttempts++
		return &claimed, nil
	}
	repository.retry = func(
		dispatchID string,
		expectedVersion int64,
		workerID string,
		retryAt time.Time,
		cause string,
	) (*protocol.ExecutionDispatch, error) {
		if dispatchID != candidate.ID || expectedVersion != 8 || workerID != "worker-1" ||
			!retryAt.Equal(fixedNow.Add(time.Second)) ||
			cause != "Room runtime is unavailable" {
			t.Fatalf(
				"retry = %q/%d/%q/%s/%q",
				dispatchID,
				expectedVersion,
				workerID,
				retryAt,
				cause,
			)
		}
		pending := candidate
		pending.Status = protocol.ExecutionDispatchStatusPending
		pending.Version = 9
		return &pending, nil
	}
	service := NewService(repository)
	service.now = func() time.Time { return fixedNow }
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		context.Context,
		ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		return ExecutionDispatchReceipt{}, errors.New("Room runtime is unavailable")
	}))

	result, err := service.DispatchPending(context.Background(), "worker-1", 8)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Delivered != 0 || result.Retried != 1 {
		t.Fatalf("DispatchRunResult = %+v", result)
	}
}

func TestDispatchPendingCancelsPermanentlyStaleWorkContract(t *testing.T) {
	snapshot := dispatchDeliverySnapshot()
	snapshot.Attempts = nil
	candidate := protocol.ExecutionDispatch{
		ID:            "dispatch-stale",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "Deliver the evidence set.",
		Version:       7,
	}
	repository := &dispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionDispatch{candidate},
	}
	repository.claim = func(
		_ string,
		_ int64,
		_ string,
		_ time.Duration,
	) (*protocol.ExecutionDispatch, error) {
		claimed := candidate
		claimed.Status = protocol.ExecutionDispatchStatusClaimed
		claimed.Version++
		claimed.DeliveryAttempts++
		return &claimed, nil
	}
	repository.cancel = func(
		dispatchID string,
		expectedVersion int64,
		workerID string,
		cause string,
	) (*protocol.ExecutionDispatch, error) {
		if dispatchID != candidate.ID ||
			expectedVersion != 8 ||
			workerID != "worker-1" ||
			cause != "dispatch has no current root attempt" {
			t.Fatalf(
				"cancel = %q/%d/%q/%q",
				dispatchID,
				expectedVersion,
				workerID,
				cause,
			)
		}
		cancelled := candidate
		cancelled.Status = protocol.ExecutionDispatchStatusCancelled
		cancelled.Version = 9
		return &cancelled, nil
	}
	service := NewService(repository)
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		context.Context,
		ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		t.Fatal("stale WorkContract must not reach Room delivery")
		return ExecutionDispatchReceipt{}, nil
	}))

	result, err := service.DispatchPending(context.Background(), "worker-1", 8)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 ||
		result.Delivered != 0 ||
		result.Retried != 0 ||
		result.Cancelled != 1 {
		t.Fatalf("DispatchRunResult = %+v", result)
	}
}

func TestDispatchPendingCancelsPermanentConsumerAdmissionFailure(t *testing.T) {
	snapshot := dispatchDeliverySnapshot()
	candidate := protocol.ExecutionDispatch{
		ID:            "dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "Deliver the evidence set.",
		Version:       7,
	}
	repository := &dispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionDispatch{candidate},
	}
	repository.claim = func(
		_ string,
		_ int64,
		_ string,
		_ time.Duration,
	) (*protocol.ExecutionDispatch, error) {
		claimed := candidate
		claimed.Status = protocol.ExecutionDispatchStatusClaimed
		claimed.Version++
		claimed.DeliveryAttempts++
		return &claimed, nil
	}
	repository.cancel = func(
		_ string,
		_ int64,
		_ string,
		cause string,
	) (*protocol.ExecutionDispatch, error) {
		if cause != "target is no longer a Room member" {
			t.Fatalf("cancel cause = %q", cause)
		}
		cancelled := candidate
		cancelled.Status = protocol.ExecutionDispatchStatusCancelled
		return &cancelled, nil
	}
	service := NewService(repository)
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		context.Context,
		ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		return ExecutionDispatchReceipt{}, PermanentExecutionDispatchDelivery(
			errors.New("target is no longer a Room member"),
		)
	}))

	result, err := service.DispatchPending(context.Background(), "worker-1", 8)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cancelled != 1 || result.Retried != 0 || result.Delivered != 0 {
		t.Fatalf("DispatchRunResult = %+v", result)
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
