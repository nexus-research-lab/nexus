package goal

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestBindExplicitExecutionIsIdempotentAndFenced(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:explicit-binding",
		Objective:  "Ship the verified report",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: protocol.ExplicitGoalReservedExecutionID(
				"explicit-command-1",
			),
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateReserved,
			),
			protocol.GoalMetadataExplicitCommand:  "explicit-command-1",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := ExplicitExecutionBinding{
		GoalID:                    created.ID,
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
		ExecutionID:               protocol.ExplicitGoalReservedExecutionID("explicit-command-1"),
		CompletionCriteria:        []string{" report accepted ", "tests pass"},
		RoundID:                   "round-plan",
	}
	bound, err := service.BindExplicitExecution(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(
		bound.Metadata,
		protocol.GoalMetadataExecutionID,
	); got != input.ExecutionID {
		t.Fatalf("execution binding = %q", got)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*bound); got !=
		protocol.GoalExecutionBindingStatePending {
		t.Fatalf("execution binding state = %q, want pending", got)
	}
	if bound.Version != created.Version+1 {
		t.Fatalf("bound version = %d, want %d", bound.Version, created.Version+1)
	}
	criteria := goalMetadataStrings(
		bound.Metadata,
		protocol.GoalMetadataCompletionCriteria,
	)
	if len(criteria) != 2 || criteria[0] != "report accepted" || criteria[1] != "tests pass" {
		t.Fatalf("completion criteria = %#v", criteria)
	}

	replayed, err := service.BindExplicitExecution(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != bound.Version {
		t.Fatalf("idempotent replay advanced version from %d to %d", bound.Version, replayed.Version)
	}
	pendingEvents := 0
	for _, event := range repo.events {
		if event.EventType == "execution_binding_pending" {
			pendingEvents++
		}
	}
	if pendingEvents != 1 {
		t.Fatalf("execution_binding_pending events = %d, want 1", pendingEvents)
	}

	input.ExecutionID = "execution-other"
	_, err = service.BindExplicitExecution(context.Background(), input)
	if !errors.Is(err, ErrGoalExecutionBindingConflict) {
		t.Fatalf("conflicting rebind error = %v", err)
	}
}

func TestConfirmExplicitExecutionBindingWithoutObjectiveTransition(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:initial-binding-confirmation",
		Objective:  "Confirm the existing Execution binding",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-current",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStatePending,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	confirmed, err := service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		created.ID,
		created.ObjectiveRevision(),
		"execution-current",
		[]string{" accepted ", "tests pass"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*confirmed); got !=
		protocol.GoalExecutionBindingStateConfirmed {
		t.Fatalf("binding state = %q, want confirmed", got)
	}
	if got := goalMetadataStrings(confirmed.Metadata, protocol.GoalMetadataCompletionCriteria); !slices.Equal(got, []string{"accepted", "tests pass"}) {
		t.Fatalf("completion criteria = %#v", got)
	}
	if _, transitioning := ObjectiveTransitionFromGoal(*confirmed); transitioning {
		t.Fatalf("initial binding unexpectedly created objective transition: %#v", confirmed.Metadata)
	}
	events := 0
	for _, event := range repo.events {
		if event.EventType == "execution_bound" {
			events++
		}
	}
	if events != 1 {
		t.Fatalf("execution_bound events = %d, want 1", events)
	}

	replayed, err := service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		confirmed.ID,
		confirmed.ObjectiveRevision(),
		"execution-current",
		[]string{"accepted", "tests pass"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != confirmed.Version {
		t.Fatalf("confirmation replay advanced version from %d to %d", confirmed.Version, replayed.Version)
	}
}

func TestConfirmExplicitExecutionBindingRejectsFutureReservation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:future-reservation",
		Objective:  "Wait for authoritative materialization",
		CreatedBy:  "model",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-future",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateReserved,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		created.ID,
		created.ObjectiveRevision(),
		"execution-future",
		nil,
	)
	if !errors.Is(err, ErrGoalExecutionBindingConflict) {
		t.Fatalf("reserved confirmation error = %v, want binding conflict", err)
	}
	stored, loadErr := repo.GetGoal(context.Background(), created.ID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*stored); got !=
		protocol.GoalExecutionBindingStateReserved {
		t.Fatalf("reserved binding state changed to %q", got)
	}
}

func TestBindExplicitExecutionRejectsRetargetedGoalRevision(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:retarget-binding",
		Objective:  "Original objective",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit-command-2",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.goals[created.ID] = protocol.Goal{
		ID:         created.ID,
		SessionKey: created.SessionKey,
		Objective:  "Retargeted objective",
		Status:     protocol.GoalStatusActive,
		Version:    created.Version + 1,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(2),
			protocol.GoalMetadataExplicitCommand:   "explicit-command-2",
			protocol.GoalMetadataActivationOrigin:  string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason:  string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	_, err = service.BindExplicitExecution(context.Background(), ExplicitExecutionBinding{
		GoalID:                    created.ID,
		ExpectedObjectiveRevision: 1,
		ExecutionID:               "execution-1",
	})
	if !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("retargeted binding error = %v, want revision stale", err)
	}
}

func TestReservedExplicitGoalCompletionDoesNotRequireExecutionAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:missing-execution-audit",
		Objective:  "Complete only after WorkGraph acceptance",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit-command-3",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(context.Background(), created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-1",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("Goal status = %q, want complete", completed.Status)
	}
}

func TestConfirmedGoalCompletionFailsClosedWithoutExecutionAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:confirmed-missing-execution-audit",
		Objective:  "Complete only after WorkGraph acceptance",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-confirmed",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CompleteByModel(context.Background(), created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-1",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("CompleteByModel() error = %v, want fail-closed audit rejection", err)
	}
}

type ownerBindingReadResolver struct {
	calls      int
	resolution protocol.GoalExecutionBindingResolution
}

func (r *ownerBindingReadResolver) ResolveGoalExecutionBinding(
	context.Context,
	protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	r.calls++
	return r.resolution, nil
}

func (*ownerBindingReadResolver) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	return "", nil
}

func TestExecutionBindingForOwnerUsesCentralResolverWithoutPersisting(t *testing.T) {
	const ownerUserID = "owner-binding-read"
	states := []protocol.GoalExecutionBindingState{
		protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved,
		protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConfirmed,
		protocol.GoalExecutionBindingStateConflict,
	}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			repository := newMemoryRepository()
			item := protocol.Goal{
				ID:         "goal-binding-" + string(state),
				SessionKey: "agent:nexus:ws:dm:binding-read",
				Objective:  "Read exact binding",
				Status:     protocol.GoalStatusActive,
				Version:    7,
				Metadata: map[string]any{
					protocol.GoalMetadataOwnerUserID:           ownerUserID,
					protocol.GoalMetadataExecutionBindingState: "confirmed",
				},
			}
			repository.goals[item.ID] = item
			before := *cloneGoal(repository.goals[item.ID])
			resolver := &ownerBindingReadResolver{
				resolution: protocol.GoalExecutionBindingResolution{
					State:       state,
					ExecutionID: "execution-exact",
				},
			}
			service := NewService(config.Config{GoalEnabled: true}, repository)
			service.SetExecutionGoalCompletionReadiness(resolver)

			resolution, err := service.ExecutionBindingForOwner(
				context.Background(),
				item.ID,
				ownerUserID,
			)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.State != state || resolution.ExecutionID != "execution-exact" {
				t.Fatalf("resolution = %#v, want state %q from central resolver", resolution, state)
			}
			if resolver.calls != 1 {
				t.Fatalf("resolver calls = %d, want 1", resolver.calls)
			}
			if !reflect.DeepEqual(repository.goals[item.ID], before) {
				t.Fatalf("binding read persisted Goal changes: before=%#v after=%#v", before, repository.goals[item.ID])
			}
		})
	}
}

func TestExecutionBindingForOwnerRejectsForeignOwnerBeforeResolution(t *testing.T) {
	repository := newMemoryRepository()
	item := protocol.Goal{
		ID:         "goal-binding-private",
		SessionKey: "agent:nexus:ws:dm:binding-private",
		Objective:  "Keep binding private",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-binding-private",
		},
	}
	repository.goals[item.ID] = item
	resolver := &ownerBindingReadResolver{}
	service := NewService(config.Config{GoalEnabled: true}, repository)
	service.SetExecutionGoalCompletionReadiness(resolver)

	_, err := service.ExecutionBindingForOwner(
		context.Background(),
		item.ID,
		"owner-attacker",
	)
	if !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("error = %v, want ErrGoalForbidden", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver calls = %d, want authorization before resolution", resolver.calls)
	}
}

type clearBindingResolver struct {
	state        protocol.GoalExecutionBindingState
	resolveCalls int
	blockerCalls int
}

func (r *clearBindingResolver) ResolveGoalExecutionBinding(
	context.Context,
	protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	r.resolveCalls++
	resolution := protocol.GoalExecutionBindingResolution{State: r.state}
	if r.state != protocol.GoalExecutionBindingStateStandalone {
		resolution.ReservedExecutionID = "execution-clear"
	}
	if r.state == protocol.GoalExecutionBindingStateConfirmed {
		resolution.ExecutionID = "execution-clear"
	}
	return resolution, nil
}

func (r *clearBindingResolver) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	r.blockerCalls++
	return "", nil
}

func TestGoalClearUsesCentralExecutionBindingPhaseAcrossEntrypoints(t *testing.T) {
	for _, entry := range []struct {
		name  string
		clear func(context.Context, *Service, protocol.Goal) (bool, error)
	}{
		{
			name: "REST clear",
			clear: func(ctx context.Context, service *Service, item protocol.Goal) (bool, error) {
				return service.Clear(ctx, item.ID)
			},
		},
		{
			name: "app-server HTTP and WebSocket clear common path",
			clear: func(ctx context.Context, service *Service, item protocol.Goal) (bool, error) {
				return service.ClearFromThreadGoalParams(ctx, goalappserver.ThreadGoalClearParams{
					ThreadID: item.SessionKey,
				})
			},
		},
	} {
		for _, phase := range []struct {
			state       protocol.GoalExecutionBindingState
			wantClear   bool
			wantInvalid bool
		}{
			{state: protocol.GoalExecutionBindingStateStandalone, wantClear: true},
			{state: protocol.GoalExecutionBindingStateReserved, wantClear: true},
			{state: protocol.GoalExecutionBindingStatePending, wantInvalid: true},
			{state: protocol.GoalExecutionBindingStateConfirmed, wantInvalid: true},
			{state: protocol.GoalExecutionBindingStateConflict, wantInvalid: true},
		} {
			t.Run(entry.name+"/"+string(phase.state), func(t *testing.T) {
				repo := newMemoryRepository()
				service := NewService(config.Config{GoalEnabled: true}, repo)
				service.nowFn = fixedClock()
				service.idFactory = sequentialID()
				resolver := &clearBindingResolver{state: phase.state}
				service.SetExecutionGoalCompletionReadiness(resolver)
				created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
					SessionKey: "agent:nexus:ws:dm:clear-" + string(phase.state),
					Objective:  "clear only when no materialized Execution is bound",
					CreatedBy:  "model",
				})
				if err != nil {
					t.Fatal(err)
				}
				cleared, err := entry.clear(context.Background(), service, *created)
				if phase.wantInvalid {
					if !errors.Is(err, ErrGoalInvalidState) || cleared {
						t.Fatalf("clear = %v, err = %v, want fail-closed invalid state", cleared, err)
					}
					stored, loadErr := repo.GetGoal(context.Background(), created.ID)
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					if stored == nil {
						t.Fatal("Goal was deleted despite a materialized or indeterminate Execution binding")
					}
				} else if err != nil || cleared != phase.wantClear {
					t.Fatalf("clear = %v, err = %v, want clear", cleared, err)
				}
				if resolver.blockerCalls != 0 {
					t.Fatalf("completion readiness calls = %d, want central binding classification only", resolver.blockerCalls)
				}
			})
		}
	}
}
