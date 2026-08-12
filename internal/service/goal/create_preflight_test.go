// INPUT: live runtime scope guard、历史 Goal 与外部 activation failure。
// OUTPUT: Goal 创建前冲突判定和 activation 错误传播回归覆盖。
// POS: Goal service runtime accounting 创建边界测试。
package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

type concurrentGoalCreateRepository struct {
	*memoryRepository
	winner protocol.Goal
}

func (r *concurrentGoalCreateRepository) CreateGoalWithEvent(
	_ context.Context,
	_ protocol.Goal,
	_ protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.goals[r.winner.ID] = r.winner
	return nil, errors.New("current Goal unique constraint")
}

func TestServiceCreatePreflightUsesModelScopeAndRunsBeforePersistence(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	accountant := &fakeExternalMutationAccountant{
		createConflicts: map[string][]string{
			"root-consumed": {"round-b", "round-a"},
		},
	}
	service.SetExternalMutationAccountant(accountant)

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-preflight-model",
		Objective:  "Must not share an already consumed scope",
		CreatedBy:  "model",
		RoundID:    " root-consumed ",
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("Create() error = %v, want ErrGoalConflict", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("preflight persisted goals=%d events=%d, want zero", len(repo.goals), len(repo.events))
	}
	if len(accountant.createPreflightCalls) != 1 ||
		accountant.createPreflightCalls[0] != "agent:nexus:ws:dm:create-preflight-model:root-consumed" {
		t.Fatalf("preflight calls = %#v, want normalized model scope", accountant.createPreflightCalls)
	}

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-preflight-model",
		Objective:  "An unrelated scope may create its Goal",
		CreatedBy:  "model",
		RoundID:    "root-other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || created.ID == "" {
		t.Fatalf("created = %#v, want Goal for unrelated scope", created)
	}
}

func TestServiceCreatePreflightUsesWholeSessionForExternalGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	accountant := &fakeExternalMutationAccountant{
		createConflicts: map[string][]string{
			"": {"round-live"},
		},
	}
	service.SetExternalMutationAccountant(accountant)

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-preflight-external",
		Objective:  "External Goal starts from now",
		CreatedBy:  "user",
		RoundID:    "ignored-for-external",
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("Create() error = %v, want ErrGoalConflict", err)
	}
	if len(accountant.createPreflightCalls) != 1 ||
		accountant.createPreflightCalls[0] != "agent:nexus:ws:dm:create-preflight-external:" {
		t.Fatalf("preflight calls = %#v, want whole-session check", accountant.createPreflightCalls)
	}
	if len(repo.goals) != 0 {
		t.Fatalf("goals = %#v, want no persistence before conflict", repo.goals)
	}
}

func TestServiceCreateClassifiesConcurrentCurrentGoalInsertAsConflict(t *testing.T) {
	sessionKey := "agent:nexus:ws:dm:create-concurrent-conflict"
	repo := &concurrentGoalCreateRepository{
		memoryRepository: newMemoryRepository(),
		winner: protocol.Goal{
			ID:         "goal-concurrent-winner",
			SessionKey: sessionKey,
			Objective:  "the concurrent winner",
			Status:     protocol.GoalStatusActive,
			Version:    1,
		},
	}
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "the losing concurrent request",
		CreatedBy:  "user",
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("Create() error = %v, want ErrGoalConflict", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("losing create events = %#v, want none", repo.events)
	}
}

func TestServiceCreateDoesNotTreatHistoricalUnfinalizedGoalAsLiveConflict(t *testing.T) {
	repo := newMemoryRepository()
	historical := protocol.Goal{
		ID:             "goal-historical",
		SessionKey:     "agent:nexus:ws:dm:create-after-history",
		Objective:      "Old Goal with unavailable evidence",
		Status:         protocol.GoalStatusComplete,
		Version:        1,
		UsageFinalized: false,
	}
	repo.goals[historical.ID] = historical
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: historical.SessionKey,
		Objective:  "A new live round may start a new Goal",
		CreatedBy:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || created.ID == historical.ID {
		t.Fatalf("created = %#v, want a new current Goal", created)
	}
}

func TestServiceCreatePropagatesActivationError(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	activationErr := errors.New("bind runtime Goal scope")
	accountant := &fakeExternalMutationAccountant{
		roundID:     "round-activated-before-error",
		activateErr: activationErr,
	}
	service.SetExternalMutationAccountant(accountant)
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-activation-error",
		Objective:  "Activation errors are visible",
		CreatedBy:  "user",
	})
	if !errors.Is(err, activationErr) {
		t.Fatalf("Create() error = %v, want activation error", err)
	}
	if len(accountant.activatedSessionKeys) != 1 {
		t.Fatalf("activation calls = %#v, want one", accountant.activatedSessionKeys)
	}
	if len(accountant.activationRollbacks) != 1 ||
		accountant.activationRollbacks[0] != "agent:nexus:ws:dm:create-activation-error:round-activated-before-error" {
		t.Fatalf("activation rollbacks = %#v, want successful runtime rounds cleared", accountant.activationRollbacks)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("failed activation left goals=%d events=%d, want rollback", len(repo.goals), len(repo.events))
	}
	if len(broadcaster.events) != 0 {
		t.Fatalf("broadcast events = %#v, failed creation must stay invisible", broadcaster.events)
	}
}

func TestServiceThreadGoalSetUsesCreatePreflight(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	accountant := &fakeExternalMutationAccountant{
		createConflicts: map[string][]string{
			"": {"room-slot-live"},
		},
	}
	service.SetExternalMutationAccountant(accountant)
	objective := "App-server must share the external creation fence"

	_, err := service.SetFromThreadGoalParams(context.Background(), goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:thread-goal-create-preflight",
		Objective: &objective,
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want ErrGoalConflict", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("preflight persisted goals=%d events=%d, want zero", len(repo.goals), len(repo.events))
	}
}

func TestServiceThreadGoalSetRollsBackActivationFailure(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	activationErr := errors.New("thread Goal runtime bind conflict")
	accountant := &fakeExternalMutationAccountant{activateErr: activationErr}
	service.SetExternalMutationAccountant(accountant)
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	objective := "App-server activation must be atomic to callers"

	_, err := service.SetFromThreadGoalParams(context.Background(), goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:thread-goal-activation-error",
		Objective: &objective,
	})
	if !errors.Is(err, activationErr) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want activation error", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 || len(broadcaster.events) != 0 {
		t.Fatalf(
			"failed activation left goals=%d events=%d broadcasts=%d, want invisible rollback",
			len(repo.goals),
			len(repo.events),
			len(broadcaster.events),
		)
	}
}
