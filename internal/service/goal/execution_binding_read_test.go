package goal

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
					protocol.GoalMetadataOwnerUserID: ownerUserID,
					// Deliberately contradictory client-visible metadata proves
					// the read result comes from the injected central resolver.
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
