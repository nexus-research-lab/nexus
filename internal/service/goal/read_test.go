package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceGoalByIDForOwnerRequiresExactDurableProvenance(t *testing.T) {
	repo := newMemoryRepository()
	repo.goals["goal-owned"] = protocol.Goal{
		ID:         "goal-owned",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-source"),
		Objective:  "coordinate across topics",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-exact",
		},
	}
	repo.goals["goal-ownerless"] = protocol.Goal{
		ID:         "goal-ownerless",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-legacy"),
		Objective:  "legacy goal without durable owner",
		Status:     protocol.GoalStatusActive,
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	item, err := service.GoalByIDForOwner(context.Background(), "goal-owned", "owner-exact")
	if err != nil || item == nil || item.ID != "goal-owned" {
		t.Fatalf("exact owner read = %+v, %v", item, err)
	}
	for name, goalID := range map[string]string{
		"foreign owner":    "goal-owned",
		"ownerless legacy": "goal-ownerless",
	} {
		t.Run(name, func(t *testing.T) {
			if item, err := service.GoalByIDForOwner(
				context.Background(), goalID, "owner-foreign",
			); item != nil || !errors.Is(err, ErrGoalForbidden) {
				t.Fatalf("GoalByIDForOwner() = %+v, %v; want forbidden", item, err)
			}
		})
	}
}
