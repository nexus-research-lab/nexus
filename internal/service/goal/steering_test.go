package goal

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceQueuesObjectiveUpdateSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Original",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedObjective := "Updated <goal>"
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &updatedObjective}); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one objective update steering item", dispatcher.items)
	}
	item := dispatcher.items[0]
	if item.sessionKey != created.SessionKey ||
		item.contextName != "goal" ||
		!strings.Contains(item.content, "active thread goal objective") ||
		!strings.Contains(item.content, "existing, tracked Goal") ||
		!strings.Contains(item.content, "Updated &lt;goal&gt;") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want escaped objective update steering", item)
	}
}

func TestServiceUpdateSameObjectiveDoesNotQueueObjectiveSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Same objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	sameObjective := "Same objective"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &sameObjective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version {
		t.Fatalf("updated version = %d, want unchanged %d", updated.Version, created.Version)
	}
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want no objective update steering", dispatcher.items)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events = %#v, want no update event for unchanged objective", repo.events)
	}
}

func TestServiceQueuesBudgetLimitSteering(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budget <work>",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one budget limit steering item", dispatcher.items)
	}
	if item := dispatcher.items[0]; item.sessionKey != created.SessionKey ||
		item.contextName != "goal" ||
		!strings.Contains(item.content, "active thread goal") ||
		!strings.Contains(item.content, "existing, tracked Goal") ||
		!strings.Contains(item.content, "reached its token budget") ||
		!strings.Contains(item.content, "<objective>") ||
		!strings.Contains(item.content, "</objective>") ||
		!strings.Contains(item.content, "budget_limited") ||
		!strings.Contains(item.content, "Budget &lt;work&gt;") ||
		strings.Contains(item.content, "<untrusted_objective>") ||
		strings.Contains(item.content, "Budget <work>") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want budget limit steering", item)
	}
}
