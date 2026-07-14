package goal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServicePlanContinuationForSession(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 3,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete parity",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.RoundID != "goal_continuation_3" {
		t.Fatalf("plan = %#v, want hidden continuation round", plan)
	}
	if !plan.HiddenFromUser || !plan.Synthetic || plan.Purpose != goalContinuationPurpose {
		t.Fatalf("plan visibility = %#v, want hidden synthetic goal continuation", plan)
	}
	for _, want := range []string{
		"Continue working toward the active thread goal.",
		"Runtime note: this is an existing, tracked Goal",
		"First compare the current state against the objective.",
		"choose the next concrete, evidence-backed step and execute it",
		"Do not ask the user which direction to take when there is an obvious next step",
		"Do not mention hidden continuations",
		"Complete parity",
		"Completion audit:",
		"Blocked audit:",
		"mcp__nexus_goal__update_goal",
		"bare `update_goal`",
		"Tokens remaining:",
	} {
		if !strings.Contains(plan.Prompt, want) {
			t.Fatalf("continuation prompt missing %q: %s", want, plan.Prompt)
		}
	}
	for _, forbidden := range []string{"active Nexus Goal", "Nexus runtime:", "PreviousRoundID:"} {
		if strings.Contains(plan.Prompt, forbidden) {
			t.Fatalf("continuation prompt contains legacy runtime wording %q: %s", forbidden, plan.Prompt)
		}
	}
	if strings.Contains(strings.ToLower(plan.Prompt), "absence of a new user message") {
		t.Fatalf("continuation prompt should not mention missing user messages: %s", plan.Prompt)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "continuation_scheduled" || repo.events[1].RoundID != plan.RoundID {
		t.Fatalf("events = %#v, want continuation_scheduled", repo.events)
	}
}

func TestServicePlanContinuationForRoomGoalIncludesLeadPrompt(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 3,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "room:group:conversation-1",
		Objective:  "完成房间协作",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalScope:         "room",
			protocol.GoalMetadataRoomGoalLeadAgentID:   "agent-host",
			protocol.GoalMetadataRoomGoalLeadAgentName: "主持人",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Room Goal lead:",
		"主持人 (agent-host)",
		"This is a shared Room Goal",
		"publish a normal public Room message that @mentions exactly that member",
		"Public @ delegation is visible to the user",
		"visible collaboration is part of completion",
		"@ exactly one non-lead member",
		"must not call the Goal update tool in that same turn",
		"only mark the Goal complete after the full room objective is verified",
	} {
		if plan == nil || !strings.Contains(plan.Prompt, want) {
			t.Fatalf("Room Goal continuation prompt missing %q: %s", want, plan.Prompt)
		}
	}
}

func TestServicePlanContinuationRetriesVersionStale(t *testing.T) {
	repo := &staleOnceVersionRepository{
		memoryRepository: newMemoryRepository(),
		mutate: func(item protocol.Goal) protocol.Goal {
			item.Objective = "Concurrent room slot update"
			return item
		},
	}
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "room:group:continuation-plan-race",
		Objective:  "Plan continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "room-round-1")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected {
		t.Fatal("stale version repository did not inject a version conflict")
	}
	if plan == nil {
		t.Fatal("plan = nil, want retried continuation")
	}
	if plan.Goal.Objective != "Concurrent room slot update" || !strings.Contains(plan.Prompt, "Concurrent room slot update") {
		t.Fatalf("plan = %#v, want continuation from reloaded room goal", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 || current.Objective != "Concurrent room slot update" {
		t.Fatalf("current = %#v, want retried continuation update on reloaded goal", current)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_scheduled" || got.RoundID != plan.RoundID {
		t.Fatalf("last event = %#v, want continuation_scheduled after retry", got)
	}
}

func TestServiceReleaseConcurrentContinuationReservationsExactlyOnce(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 3,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Finish the audit",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || second == nil || second.Goal.ContinuationCount != 2 {
		t.Fatalf("plans = %#v / %#v, want two reservations", first, second)
	}

	if _, err := service.ReleaseContinuationPlan(ctx, *first, "explicit input won"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *second, "explicit input won"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *first, "duplicate release"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 0 || len(continuationReservations(current.Metadata)) != 0 {
		t.Fatalf("current = %#v, want both reservations released exactly once", current)
	}
}

func TestServiceClaimContinuationKeepsCountAndMakesReleaseNoop(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 3,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:claim",
		Objective:  "Finish exactly once",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v err=%v", plan, err)
	}
	if _, err := service.ClaimContinuationPlan(ctx, *plan); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClaimContinuationPlan(ctx, *plan); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("duplicate claim error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *plan, "duplicate dispatch saw runtime running"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 || len(continuationReservations(current.Metadata)) != 0 {
		t.Fatalf("current = %#v, want claimed continuation counted once without pending reservation", current)
	}
	startedEvents := 0
	deferredEvents := 0
	for _, event := range repo.events {
		switch event.EventType {
		case "continuation_started":
			startedEvents++
		case "continuation_deferred":
			deferredEvents++
		}
	}
	if startedEvents != 1 || deferredEvents != 0 {
		t.Fatalf("started=%d deferred=%d, want idempotent claim and no deferred event", startedEvents, deferredEvents)
	}
}

func TestServiceClaimContinuationRejectsRetargetAfterValidation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:claim-retarget",
		Objective:  "Analyze M3 and M4",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v err=%v", plan, err)
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil || !current {
		t.Fatalf("pre-claim validation = %v err=%v, want current", current, err)
	}
	retargeted, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective:                 "Analyze M4 and M5",
		RoundID:                   "round-correction",
		ExpectedObjectiveRevision: plan.Goal.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *plan); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("claim after retarget error = %v, want ErrGoalRevisionStale", err)
	}
	currentGoal, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if currentGoal.Objective != retargeted.Objective || currentGoal.ObjectiveRevision() != retargeted.ObjectiveRevision() ||
		currentGoal.ContinuationCount != 0 || len(continuationReservations(currentGoal.Metadata)) != 0 {
		t.Fatalf("current = %#v, want retargeted Goal without stale reservation", currentGoal)
	}
	for _, event := range repo.events {
		if event.EventType == "continuation_started" && event.RoundID == plan.RoundID {
			t.Fatalf("stale continuation emitted started event: %#v", event)
		}
	}
}

func TestServiceClaimContinuationAllowsSameObjectiveVersionBump(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:claim-version",
		Objective:  "Preserve usage while starting",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v err=%v", plan, err)
	}
	used, err := service.RecordUsageForGoal(ctx, created.ID, protocol.GoalUsage{InputTokens: 4, OutputTokens: 1}, "round-usage")
	if err != nil {
		t.Fatal(err)
	}
	if used.Version == plan.Goal.Version || used.ObjectiveRevision() != plan.Goal.ObjectiveRevision() {
		t.Fatalf("usage versions = goal:%d->%d objective:%d->%d", plan.Goal.Version, used.Version, plan.Goal.ObjectiveRevision(), used.ObjectiveRevision())
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil || !current {
		t.Fatalf("validation after usage bump = %v err=%v, want current", current, err)
	}
	claimed, err := service.ClaimContinuationPlan(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Usage.InputTokens != 4 || claimed.ContinuationCount != 1 || len(continuationReservations(claimed.Metadata)) != 0 {
		t.Fatalf("claimed = %#v, want usage preserved and reservation consumed", claimed)
	}
}

func TestServiceGoalContinuationStillCurrentRejectsStaleGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Skip stale continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if !current {
		t.Fatal("GoalContinuationStillCurrent() = false, want true for current active goal")
	}
	if _, err = service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective: "Use the corrected objective",
		RoundID:   "round-correction",
	}); err != nil {
		t.Fatal(err)
	}
	current, err = service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if current {
		t.Fatal("GoalContinuationStillCurrent() = true, want false after same Goal was retargeted")
	}

	stale := repo.goals[created.ID]
	stale.Status = protocol.GoalStatusPaused
	stale.Version++
	repo.goals[created.ID] = stale
	current, err = service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if current {
		t.Fatal("GoalContinuationStillCurrent() = true, want false after goal is no longer active")
	}
}

func TestServicePlanContinuationStopsWhenBudgetExhausted(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budgeted work",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10, RuntimeSeconds: 7}, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after budget exhaustion", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusBudgetLimited || current.LastError == "" || current.TimeUsedSeconds != 7 {
		t.Fatalf("current = %#v, want budget_limited with last error and runtime", current)
	}
}

func TestServicePlanContinuationStopsAtUsageLimit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 1,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Limited work",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after usage limit", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusUsageLimited || current.LastError == "" {
		t.Fatalf("current = %#v, want usage_limited with last error", current)
	}
}

func TestServiceResumeUsageLimitedGoalStartsFreshContinuationRun(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 1,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Resume after continuation cap",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2"); err != nil {
		t.Fatal(err)
	}
	limited, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.ContinuationCount != 1 {
		t.Fatalf("limited = %#v, want usage_limited after one continuation", limited)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.ContinuationCount != 0 {
		t.Fatalf("resumed = %#v, want active with continuation count reset", resumed)
	}
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-3")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("next plan = nil, want fresh continuation after resume")
	}
	if next.Goal.ContinuationCount != 1 {
		t.Fatalf("next continuation count = %d, want 1 for fresh run", next.Goal.ContinuationCount)
	}
}
