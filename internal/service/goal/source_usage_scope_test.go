package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type capturingUsageSourceRepository struct {
	*memoryRepository
	snapshot       protocol.GoalUsageSourceSnapshot
	claim          protocol.GoalUsageSourceRoundClaim
	parentSnapshot protocol.GoalUsageParentSnapshot
	binding        protocol.GoalUsageScopeBinding
}

func (r *capturingUsageSourceRepository) ApplyUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	r.snapshot = snapshot
	return protocol.GoalUsageSourceResult{}, nil
}

func (r *capturingUsageSourceRepository) ClaimUsageSourceRound(
	_ context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	r.claim = claim
	return protocol.GoalUsageSourceResult{}, nil
}

func (r *capturingUsageSourceRepository) RecordUsageParentSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	r.parentSnapshot = snapshot
	return protocol.GoalUsageParentResult{}, nil
}

func (r *capturingUsageSourceRepository) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	r.binding = binding
	return protocol.GoalUsageScopeBindResult{}, nil
}

func TestServiceUsageSourceNormalizesDurableScopeRound(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	if _, err := service.RecordUsageSourceSnapshot(context.Background(), protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            " owner-1 ",
		RuntimeSessionKey:      " runtime-1 ",
		SourceKind:             " nxs_task ",
		SourceID:               " task-1 ",
		CumulativeActualTokens: 12,
		GoalSessionKey:         " agent:nexus:ws:dm:conversation-1 ",
		RoundID:                " round-1 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.snapshot.OwnerUserID != "owner-1" ||
		repo.snapshot.GoalSessionKey != "agent:nexus:ws:dm:conversation-1" ||
		repo.snapshot.ScopeRoundID != "round-1" {
		t.Fatalf("snapshot = %+v, want normalized durable scope", repo.snapshot)
	}

	if _, err := service.ClaimUsageSourceRound(context.Background(), protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:    " owner-1 ",
		SourceKind:     " nxs_task ",
		RoundID:        " round-2 ",
		GoalID:         " goal-1 ",
		GoalSessionKey: " agent:nexus:ws:dm:conversation-1 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.claim.ScopeRoundID != "round-2" || repo.claim.RuntimeSessionKey != "" {
		t.Fatalf("claim = %+v, want scope fallback independent of runtime session", repo.claim)
	}
}

func TestServiceUsageSourceRequiresNXSTaskScopeIdentity(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	validSnapshot := protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            "owner-1",
		RuntimeSessionKey:      "runtime-1",
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               "task-1",
		CumulativeActualTokens: 1,
		GoalSessionKey:         "agent:nexus:ws:dm:conversation-1",
		ScopeRoundID:           "scope-1",
	}
	for _, mutate := range []func(*protocol.GoalUsageSourceSnapshot){
		func(item *protocol.GoalUsageSourceSnapshot) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.SourceKind = "other" },
	} {
		item := validSnapshot
		mutate(&item)
		if _, err := service.RecordUsageSourceSnapshot(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("snapshot = %+v, error = %v, want invalid input", item, err)
		}
	}

	validClaim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:    "owner-1",
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   "scope-1",
		GoalID:         "goal-1",
		GoalSessionKey: "agent:nexus:ws:dm:conversation-1",
	}
	for _, mutate := range []func(*protocol.GoalUsageSourceRoundClaim){
		func(item *protocol.GoalUsageSourceRoundClaim) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.SourceKind = "other" },
	} {
		item := validClaim
		mutate(&item)
		if _, err := service.ClaimUsageSourceRound(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("claim = %+v, error = %v, want invalid input", item, err)
		}
	}
}

func TestServiceNormalizesRoomParentLedgerAndFromNowBinding(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	if _, err := service.RecordUsageParentSnapshot(
		context.Background(),
		protocol.GoalUsageParentSnapshot{
			OwnerUserID:        " owner-1 ",
			GoalSessionKey:     " room:group:conversation-1 ",
			ScopeRoundID:       " root-round-1 ",
			SourceRoundID:      " slot-round-1 ",
			TokenUsageObserved: true,
		},
	); err != nil {
		t.Fatal(err)
	}
	if repo.parentSnapshot.OwnerUserID != "owner-1" ||
		repo.parentSnapshot.GoalSessionKey != "room:group:conversation-1" ||
		repo.parentSnapshot.ScopeRoundID != "root-round-1" ||
		repo.parentSnapshot.SourceRoundID != "slot-round-1" ||
		repo.parentSnapshot.EventID == "" ||
		repo.parentSnapshot.ObservedAt.IsZero() {
		t.Fatalf("parent snapshot = %+v, want normalized identity and generated audit fields", repo.parentSnapshot)
	}

	if _, err := service.BindUsageScopeFromNow(
		context.Background(),
		protocol.GoalUsageScopeBinding{
			OwnerUserID:    " owner-1 ",
			GoalSessionKey: " room:group:conversation-1 ",
			ScopeRoundID:   " root-round-1 ",
			GoalID:         " goal-1 ",
		},
	); err != nil {
		t.Fatal(err)
	}
	if repo.binding.OwnerUserID != "owner-1" ||
		repo.binding.GoalSessionKey != "room:group:conversation-1" ||
		repo.binding.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
		repo.binding.ScopeRoundID != "root-round-1" ||
		repo.binding.GoalID != "goal-1" ||
		repo.binding.BoundAt.IsZero() {
		t.Fatalf("from-now binding = %+v, want normalized durable scope", repo.binding)
	}
}

func TestServiceRequiresRoomParentLedgerIdentity(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	valid := protocol.GoalUsageParentSnapshot{
		OwnerUserID:        "owner-1",
		GoalSessionKey:     "room:group:conversation-1",
		ScopeRoundID:       "root-round-1",
		SourceRoundID:      "slot-round-1",
		TokenUsageObserved: true,
	}
	for _, mutate := range []func(*protocol.GoalUsageParentSnapshot){
		func(item *protocol.GoalUsageParentSnapshot) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.SourceRoundID = "" },
	} {
		item := valid
		mutate(&item)
		if _, err := service.RecordUsageParentSnapshot(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("parent snapshot = %+v, error = %v, want invalid input", item, err)
		}
	}
}
