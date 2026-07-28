package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type atomicUsageScopeMemoryRepository struct {
	*memoryRepository
	binding      protocol.GoalUsageScopeBinding
	createdEvent protocol.GoalEvent
	atomicCalls  int
	appendCalls  int
}

func (r *atomicUsageScopeMemoryRepository) AppendEvent(ctx context.Context, event protocol.GoalEvent) error {
	r.appendCalls++
	return r.memoryRepository.AppendEvent(ctx, event)
}

func (r *atomicUsageScopeMemoryRepository) CreateGoalWithUsageScope(
	ctx context.Context,
	item protocol.Goal,
	createdEvent protocol.GoalEvent,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeCreateResult, error) {
	r.atomicCalls++
	r.binding = binding
	r.createdEvent = createdEvent
	created, err := r.memoryRepository.CreateGoal(ctx, item)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	created.Usage = protocol.GoalUsage{
		ActualTotalTokens: 13,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}
	created.Version++
	r.goals[created.ID] = *created
	usageEvent := protocol.GoalEvent{
		ID:         binding.UsageEventID,
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    binding.ScopeRoundID,
		Payload:    map[string]any{"actual_tokens": int64(13)},
		CreatedAt:  binding.BoundAt,
	}
	r.events = append(r.events, createdEvent, usageEvent)
	return protocol.GoalUsageScopeCreateResult{
		Goal:            created,
		UsageEvent:      &usageEvent,
		AttributedDelta: 13,
	}, nil
}

func TestServiceCreateAtomicallyBindsModelGoalUsageScope(t *testing.T) {
	for _, test := range []struct {
		name             string
		requestOwner     string
		wantBindingOwner string
	}{
		{
			name:             "request owner wins",
			requestOwner:     "owner-request",
			wantBindingOwner: "owner-request",
		},
		{
			name:             "authenticated owner fallback",
			wantBindingOwner: "owner-context",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &atomicUsageScopeMemoryRepository{memoryRepository: newMemoryRepository()}
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			broadcaster := &fakeGoalBroadcaster{}
			service.SetEventBroadcaster(broadcaster)
			ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-context"})

			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:conversation-1",
				Objective:   "Recover child usage after restart",
				CreatedBy:   "model",
				RoundID:     " scope-round-1 ",
				OwnerUserID: test.requestOwner,
			})
			if err != nil {
				t.Fatal(err)
			}
			if repo.atomicCalls != 1 || repo.appendCalls != 0 {
				t.Fatalf("atomic calls = %d, append calls = %d, want 1 and 0", repo.atomicCalls, repo.appendCalls)
			}
			if len(repo.events) != 2 ||
				repo.events[0].EventType != "created" ||
				repo.events[1].EventType != "usage_recorded" {
				t.Fatalf("stored events = %+v, want one created and one usage event", repo.events)
			}
			if repo.binding.OwnerUserID != test.wantBindingOwner ||
				repo.binding.GoalSessionKey != created.SessionKey ||
				repo.binding.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
				repo.binding.ScopeRoundID != "scope-round-1" ||
				repo.binding.GoalID != created.ID ||
				repo.binding.UsageEventID == "" {
				t.Fatalf("scope binding = %+v, want durable model round binding", repo.binding)
			}
			if repo.createdEvent.ID == "" || repo.createdEvent.ID == repo.binding.UsageEventID {
				t.Fatalf("created event = %+v, binding = %+v, want distinct event identities", repo.createdEvent, repo.binding)
			}
			if created.Usage.ActualTokens() != 13 {
				t.Fatalf("created usage = %+v, want atomic pending attribution", created.Usage)
			}
			if len(broadcaster.events) != 2 ||
				broadcaster.events[0].EventType != protocol.EventTypeGoalCreated ||
				broadcaster.events[1].EventType != protocol.EventTypeGoalProgress {
				t.Fatalf("broadcast events = %+v, want created then usage progress", broadcaster.events)
			}
			broadcastGoal, ok := broadcaster.events[0].Data["goal"].(protocol.Goal)
			if !ok || broadcastGoal.Usage.ActualTokens() != 13 {
				t.Fatalf("created broadcast goal = %+v, want latest atomically attributed usage", broadcaster.events[0].Data["goal"])
			}
		})
	}
}
