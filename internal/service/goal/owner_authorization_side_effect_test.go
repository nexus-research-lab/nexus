package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestForeignOwnerUpdateAndResumeAuthorizeBeforeSideEffects(t *testing.T) {
	const (
		goalOwner    = "owner-victim"
		foreignOwner = "owner-attacker"
	)
	for _, test := range []struct {
		name   string
		status protocol.GoalStatus
		mutate func(context.Context, *Service, protocol.Goal) error
	}{
		{
			name:   "REST Update",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				objective := "attacker objective"
				_, err := service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
					Objective:   &objective,
					OwnerUserID: foreignOwner,
				})
				return err
			},
		},
		{
			name:   "REST Resume",
			status: protocol.GoalStatusPaused,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Resume(ctx, item.ID)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			item := protocol.Goal{
				ID:         "goal-victim",
				SessionKey: "agent:victim-agent:nexus:dm:victim-thread",
				Objective:  "victim objective",
				Status:     test.status,
				Version:    1,
				Metadata: map[string]any{
					protocol.GoalMetadataOwnerUserID: goalOwner,
				},
			}
			repo.goals[item.ID] = item
			service := NewService(config.Config{
				GoalEnabled:             true,
				GoalAutoContinueEnabled: true,
			}, repo)
			accountant := &fakeExternalMutationAccountant{
				service: service,
				usage:   protocol.GoalUsage{TotalTokens: 17},
				roundID: "round-attacker",
			}
			service.SetExternalMutationAccountant(accountant)
			dispatcher := &fakeContinuationDispatcher{}
			service.SetContinuationDispatcher(dispatcher)
			ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
				UserID: foreignOwner,
				Role:   authctx.RoleOwner,
			})

			err := test.mutate(ctx, service, item)
			if !errors.Is(err, ErrGoalForbidden) {
				t.Fatalf("mutation error = %v, want ErrGoalForbidden", err)
			}
			stored := repo.goals[item.ID]
			if len(accountant.sessionKeys) != 0 {
				t.Fatalf("accounting flushes = %#v, want none", accountant.sessionKeys)
			}
			if stored.Usage != (protocol.GoalUsage{}) || stored.Version != item.Version ||
				stored.Status != item.Status || stored.Objective != item.Objective {
				t.Fatalf("stored Goal = %#v, want unchanged %#v", stored, item)
			}
			if len(repo.events) != 0 {
				t.Fatalf("events = %#v, want none", repo.events)
			}
			if len(dispatcher.plans) != 0 || dispatcher.deferCalls != 0 {
				t.Fatalf("dispatcher plans=%d deferCalls=%d, want no calls", len(dispatcher.plans), dispatcher.deferCalls)
			}
		})
	}
}
