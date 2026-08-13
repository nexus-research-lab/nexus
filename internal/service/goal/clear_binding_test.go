package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

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
				} else {
					if err != nil || cleared != phase.wantClear {
						t.Fatalf("clear = %v, err = %v, want clear", cleared, err)
					}
				}
				if resolver.blockerCalls != 0 {
					t.Fatalf("completion readiness calls = %d, want central binding classification only", resolver.blockerCalls)
				}
			})
		}
	}
}
