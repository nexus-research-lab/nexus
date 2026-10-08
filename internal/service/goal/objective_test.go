package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceCreateGoalFallsBackWhenRewriteFails(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetObjectiveRewriter(fakeObjectiveRewriter{})

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "保留原始目标",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != "保留原始目标" {
		t.Fatalf("objective = %q, want fallback original", created.Objective)
	}
	if got := protocol.GoalMetadataString(
		created.Metadata,
		protocol.GoalMetadataOwnerUserID,
	); got != "__system__" || len(created.Metadata) != 1 {
		t.Fatalf("metadata = %#v, want only server-owned system owner", created.Metadata)
	}
}

type fakeObjectiveRewriter struct {
	rewritten string
}

func (f fakeObjectiveRewriter) RewriteGoalObjective(context.Context, string, string, string) (string, error) {
	return f.rewritten, nil
}
