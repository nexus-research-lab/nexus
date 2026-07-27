package goal

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryRecordGoalUsagePersistsAggregateAndEventsAtomically(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-usage-atomic",
		SessionKey: "agent:nexus:ws:dm:usage-atomic",
		Objective:  "record exact usage",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}

	next := *created
	next.Usage = protocol.GoalUsage{
		InputTokens:       10,
		OutputTokens:      2,
		ActualTotalTokens: 42,
		ActualTotalKnown:  true,
	}.NormalizeTotals()
	next.TimeUsedSeconds = 3
	next.Version++
	next.UpdatedAt = now.Add(time.Second)
	event := protocol.GoalEvent{
		ID:         "event-usage-atomic",
		GoalID:     next.ID,
		SessionKey: next.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    "round-1",
		Payload:    map[string]any{"usage": next.Usage},
		CreatedAt:  next.UpdatedAt,
	}

	updated, err := repository.RecordGoalUsage(ctx, next, created.Version, []protocol.GoalEvent{event})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Usage.BudgetTokens() != 12 || updated.Usage.ActualTokens() != 42 ||
		updated.TimeUsedSeconds != 3 || updated.Version != 2 {
		t.Fatalf("updated = %#v, want atomic exact usage", updated)
	}
	events, err := repository.ListEvents(ctx, next.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("events = %#v, want usage event", events)
	}
}

func TestRepositoryRecordGoalUsageRollsBackAggregateWhenEventFails(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 30, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-usage-rollback",
		SessionKey: "agent:nexus:ws:dm:usage-rollback",
		Objective:  "retry without duplication",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := protocol.GoalEvent{
		ID:         "event-usage-duplicate",
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  now,
	}
	if err := repository.AppendEvent(ctx, duplicate); err != nil {
		t.Fatal(err)
	}

	next := *created
	next.Usage = protocol.GoalUsage{
		InputTokens:       10,
		OutputTokens:      2,
		ActualTotalTokens: 42,
		ActualTotalKnown:  true,
	}.NormalizeTotals()
	next.Version++
	next.UpdatedAt = now.Add(time.Second)
	duplicate.EventType = "usage_recorded"
	if _, err := repository.RecordGoalUsage(
		ctx,
		next,
		created.Version,
		[]protocol.GoalEvent{duplicate},
	); err == nil {
		t.Fatal("RecordGoalUsage() error = nil, want duplicate event failure")
	}

	stored, err := repository.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Usage.ActualTokens() != 0 || stored.Usage.BudgetTokens() != 0 ||
		stored.Version != created.Version {
		t.Fatalf("stored = %#v, want aggregate update rolled back with event", stored)
	}
}
