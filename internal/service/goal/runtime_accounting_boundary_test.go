package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestServicePauseKeepsRunningRoundForInterruptedTerminal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:boundary-pause",
		Objective:  "Pause precisely",
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{roundID: "round-running"}
	service.SetExternalMutationAccountant(accountant)

	if _, err = service.Pause(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if len(accountant.settlementBoundaries) != 1 || accountant.settlementBoundaries[0] {
		t.Fatalf("settlement boundaries = %#v, pause must await interrupted terminal", accountant.settlementBoundaries)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("cleared sessions = %#v, pause must preserve the running binding", accountant.clearedSessionKeys)
	}
	if len(accountant.finalizingSessionKeys) != 1 ||
		accountant.finalizingSessionKeys[0] != created.SessionKey {
		t.Fatalf("finalizing sessions = %#v, want current running session", accountant.finalizingSessionKeys)
	}
}

func TestServiceActiveUpdateKeepsOrdinaryCheckpoint(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:boundary-active-update",
		Objective:  "Keep terminal reconciliation",
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{roundID: "round-running"}
	service.SetExternalMutationAccountant(accountant)
	replacement := "Keep the same Goal binding"

	if _, err = service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Objective: &replacement,
	}); err != nil {
		t.Fatal(err)
	}
	if len(accountant.settlementBoundaries) != 1 || accountant.settlementBoundaries[0] {
		t.Fatalf("settlement boundaries = %#v, active update must await terminal", accountant.settlementBoundaries)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("cleared sessions = %#v, active update must not clear", accountant.clearedSessionKeys)
	}
}

func TestServiceExternalCompleteKeepsRunningRoundForTerminalFinalization(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:external-complete-running",
		Objective:  "Finish through app-server",
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{roundID: "round-running"}
	service.SetExternalMutationAccountant(accountant)
	complete := goalappserver.ThreadGoalStatusComplete

	completed, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID: created.SessionKey,
		Status:   &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || completed.UsageFinalized {
		t.Fatalf("completed = %#v, want complete awaiting terminal usage", completed)
	}
	if len(accountant.settlementBoundaries) != 1 || accountant.settlementBoundaries[0] {
		t.Fatalf("settlement boundaries = %#v, completion must await provider terminal", accountant.settlementBoundaries)
	}
	if len(accountant.finalizingSessionKeys) != 1 ||
		accountant.finalizingSessionKeys[0] != created.SessionKey {
		t.Fatalf("finalizing sessions = %#v, want current running session", accountant.finalizingSessionKeys)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("cleared sessions = %#v, completion must preserve fixed Goal binding", accountant.clearedSessionKeys)
	}
}

func TestServiceCreatesCompletedExternalGoalWithFinalizedZeroUsage(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	complete := goalappserver.ThreadGoalStatusComplete
	objective := "Already complete"

	completed, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:external-complete-idle",
		Objective: &objective,
		Status:    &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !completed.UsageFinalized || completed.Usage.ActualTokens() != 0 {
		t.Fatalf("completed = %#v, want authoritative finalized zero usage", completed)
	}
}

func TestServiceExternalCompleteIdleGoalReturnsFinalizedProjection(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:external-complete-existing-idle",
		Objective:  "Complete after creation",
	})
	if err != nil {
		t.Fatal(err)
	}
	complete := goalappserver.ThreadGoalStatusComplete

	completed, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID: created.SessionKey,
		Status:   &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.UsageFinalized || stored == nil || !stored.UsageFinalized {
		t.Fatalf("returned=%#v stored=%#v, want matching finalized projections", completed, stored)
	}
}

func testConfig() config.Config {
	return config.Config{GoalEnabled: true}
}
