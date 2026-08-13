package goal

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryCreateGoalWithUsageScopeClaimsRoomParentLedgerExactlyOnce(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 17, 0, 0, 0, time.UTC)
	sessionKey := "room:group:parent-ledger-create"
	scopeID := "root-parent-ledger-create"

	parent := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-before-handoff",
		protocol.GoalUsage{
			InputTokens:          10,
			OutputTokens:         2,
			CacheReadInputTokens: 5,
			ActualTotalTokens:    30,
			RuntimeSeconds:       7,
		},
		true,
		now,
	)
	if result, err := repository.RecordUsageParentSnapshot(ctx, parent); err != nil {
		t.Fatal(err)
	} else if result.Goal != nil || result.AttributedUsage.ActualTokens() != 0 {
		t.Fatalf("open parent snapshot = %#v, want pending only", result)
	}
	explicitZero := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-explicit-zero",
		protocol.GoalUsage{},
		true,
		now.Add(time.Minute),
	)
	if _, err := repository.RecordUsageParentSnapshot(ctx, explicitZero); err != nil {
		t.Fatal(err)
	}
	child := scopeTestSnapshot(
		"owner-a",
		"agent:child:room",
		"task-before-handoff",
		9,
		sessionKey,
		"slot-before-handoff",
		scopeID,
		now.Add(2*time.Minute),
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, child); err != nil {
		t.Fatal(err)
	}

	goal := scopeTestGoal("goal-parent-ledger-create", sessionKey, now.Add(3*time.Minute))
	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goal.ID,
		BoundAt:        goal.CreatedAt,
		UsageEventID:   "event-parent-ledger-create-usage",
	}
	result, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-parent-ledger-create", scopeID),
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Goal == nil ||
		result.Goal.Usage.InputTokens != 10 ||
		result.Goal.Usage.OutputTokens != 2 ||
		result.Goal.Usage.CacheReadInputTokens != 5 ||
		result.Goal.Usage.BudgetTokens() != 12 ||
		result.Goal.Usage.ActualTokens() != 39 ||
		result.Goal.TimeUsedSeconds != 7 ||
		result.Goal.Version != 2 ||
		result.AttributedDelta != 9 ||
		result.TokenUsageUnavailable ||
		result.UsageEvent == nil {
		t.Fatalf("atomic parent+child claim = %#v", result)
	}

	parent.GoalID = goal.ID
	parent.EventID = "event-parent-ledger-retry"
	replayed, err := repository.RecordUsageParentSnapshot(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Event != nil || replayed.AttributedUsage.ActualTokens() != 0 {
		t.Fatalf("parent replay = %#v, want no new attribution", replayed)
	}
	stored, err := repository.GetGoal(ctx, goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 39 || stored.Version != 2 {
		t.Fatalf("goal after parent replay = %#v, want actual=39 v2", stored)
	}

	var attributedRows, unavailableRows int64
	if err := repository.db.QueryRow(
		`SELECT
		    COALESCE(SUM(CASE WHEN usage_attributed THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN NOT token_usage_observed THEN 1 ELSE 0 END), 0)
		 FROM goal_usage_parent_ledger
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
	).Scan(&attributedRows, &unavailableRows); err != nil {
		t.Fatal(err)
	}
	if attributedRows != 2 || unavailableRows != 0 {
		t.Fatalf("parent ledger attributed/unavailable = %d/%d, want 2/0", attributedRows, unavailableRows)
	}
}

func TestRepositoryBoundRoomParentTerminalIsExactlyOnce(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 17, 30, 0, 0, time.UTC)
	sessionKey := "room:group:parent-ledger-bound"
	scopeID := "root-parent-ledger-bound"
	goal := scopeTestGoal("goal-parent-ledger-bound", sessionKey, now)
	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goal.ID,
		BoundAt:        now,
		UsageEventID:   "event-parent-ledger-bound-create-usage",
	}
	if _, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-parent-ledger-bound-create", scopeID),
		binding,
	); err != nil {
		t.Fatal(err)
	}

	snapshot := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-terminal",
		protocol.GoalUsage{
			InputTokens:       6,
			OutputTokens:      4,
			ActualTotalTokens: 25,
			RuntimeSeconds:    3,
		},
		true,
		now.Add(time.Minute),
	)
	snapshot.GoalID = goal.ID
	snapshot.EventID = "event-parent-ledger-bound-terminal"
	first, err := repository.RecordUsageParentSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Goal == nil || first.Goal.Usage.BudgetTokens() != 10 ||
		first.Goal.Usage.ActualTokens() != 25 || first.Goal.Version != 2 ||
		first.Event == nil {
		t.Fatalf("first bound parent result = %#v", first)
	}

	snapshot.EventID = "event-parent-ledger-bound-terminal-retry"
	second, err := repository.RecordUsageParentSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if second.Event != nil || second.AttributedUsage.ActualTokens() != 0 {
		t.Fatalf("bound parent retry = %#v, want no-op", second)
	}
	stored, err := repository.GetGoal(ctx, goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 25 || stored.Version != 2 {
		t.Fatalf("goal after bound retry = %#v, want actual=25 v2", stored)
	}
}

func TestRepositoryRoomGoalAggregatesEveryParentWithContradictoryZeroTotals(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	sessionKey := "room:group:all-agent-zero-repair"
	scopeID := "root-all-agent-zero-repair"

	rows := []protocol.GoalUsage{
		{InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 80, ActualTotalKnown: true},
		{InputTokens: 50, OutputTokens: 10, CacheReadInputTokens: 40, ActualTotalKnown: true},
	}
	for index, usage := range rows {
		snapshot := parentUsageSnapshot(
			"owner-a",
			sessionKey,
			scopeID,
			fmt.Sprintf("slot-%d", index+1),
			usage,
			true,
			now.Add(time.Duration(index)*time.Second),
		)
		if _, err := repository.RecordUsageParentSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
		// Reproduce rows written before this repair: breakdown survived while
		// the provider zero was persisted as the authoritative actual total.
		if _, err := repository.db.Exec(`UPDATE goal_usage_parent_ledger
			SET token_used_actual_total = 0, token_used_actual_estimated = 0
			WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ? AND source_round_id = ?`,
			"owner-a", sessionKey, scopeID, snapshot.SourceRoundID,
		); err != nil {
			t.Fatal(err)
		}
	}

	goal := scopeTestGoal("goal-all-agent-zero-repair", sessionKey, now.Add(time.Minute))
	result, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-all-agent-zero-repair", scopeID),
		protocol.GoalUsageScopeBinding{
			OwnerUserID:    "owner-a",
			GoalSessionKey: sessionKey,
			SourceKind:     protocol.GoalUsageSourceKindNXSTask,
			ScopeRoundID:   scopeID,
			GoalID:         goal.ID,
			BoundAt:        goal.CreatedAt,
			UsageEventID:   "event-all-agent-zero-repair-usage",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Goal == nil || result.Goal.Usage.ActualTokens() != 300 ||
		result.Goal.Usage.BudgetTokens() != 180 ||
		!result.Goal.Usage.ActualTokensAreEstimated() {
		t.Fatalf("all-Agent aggregate = %#v, want estimated actual=300 budget=180", result)
	}
}

func TestRepositoryBindUsageScopeFromNowDiscardsPreBindingBacklog(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)
	sessionKey := "room:group:parent-ledger-from-now"
	scopeID := "root-parent-ledger-from-now"
	goalID := "goal-parent-ledger-from-now"
	createUsageSourceTestGoal(t, repository, goalID, sessionKey, 0, now)

	child := scopeTestSnapshot(
		"owner-a",
		"agent:child:from-now",
		"task-from-now",
		40,
		sessionKey,
		"slot-from-now",
		scopeID,
		now.Add(time.Minute),
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, child); err != nil {
		t.Fatal(err)
	}
	parent := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-from-now",
		protocol.GoalUsage{
			InputTokens:       20,
			OutputTokens:      5,
			ActualTotalTokens: 50,
			RuntimeSeconds:    8,
		},
		true,
		now.Add(time.Minute),
	)
	if _, err := repository.RecordUsageParentSnapshot(ctx, parent); err != nil {
		t.Fatal(err)
	}

	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goalID,
		BoundAt:        now.Add(2 * time.Minute),
	}
	bound, err := repository.BindUsageScopeFromNow(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if bound.DiscardedChildPending != 1 || bound.DiscardedParentPending != 1 {
		t.Fatalf("BindUsageScopeFromNow() = %#v, want one child and one parent discarded", bound)
	}
	stored, err := repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 0 || stored.Usage.BudgetTokens() != 0 {
		t.Fatalf("goal after from-now bind = %#v, want zero usage", stored)
	}

	child.CumulativeActualTokens = 70
	child.GoalID = goalID
	child.EventID = "event-from-now-child"
	child.ObservedAt = now.Add(3 * time.Minute)
	childResult, err := repository.ApplyUsageSourceSnapshot(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	if childResult.AttributedDelta != 30 {
		t.Fatalf("post-bind child result = %#v, want current delta 30", childResult)
	}
	parent.GoalID = goalID
	parent.EventID = "event-from-now-old-parent-replay"
	parent.ObservedAt = now.Add(4 * time.Minute)
	replayedParent, err := repository.RecordUsageParentSnapshot(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if replayedParent.Event != nil || replayedParent.AttributedUsage.ActualTokens() != 0 {
		t.Fatalf("discarded parent replay = %#v, want no-op", replayedParent)
	}

	currentParent := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-current-after-bind",
		protocol.GoalUsage{
			InputTokens:       8,
			OutputTokens:      2,
			ActualTotalTokens: 20,
			RuntimeSeconds:    3,
		},
		true,
		now.Add(5*time.Minute),
	)
	currentParent.GoalID = goalID
	currentParent.EventID = "event-from-now-parent"
	parentResult, err := repository.RecordUsageParentSnapshot(ctx, currentParent)
	if err != nil {
		t.Fatal(err)
	}
	if parentResult.AttributedUsage.ActualTokens() != 20 {
		t.Fatalf("post-bind parent result = %#v, want actual delta 20", parentResult)
	}
	var discarded bool
	if err := repository.db.QueryRow(
		`SELECT discarded FROM goal_usage_parent_ledger
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ? AND source_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
		"slot-from-now",
	).Scan(&discarded); err != nil {
		t.Fatal(err)
	}
	if !discarded {
		t.Fatal("pre-binding parent row discarded = false, want durable tombstone")
	}
	stored, err = repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 50 ||
		stored.Usage.BudgetTokens() != 10 || stored.TimeUsedSeconds != 3 {
		t.Fatalf("goal after post-bind usage = %#v, want only child=30 parent=20", stored)
	}
}

func TestRepositoryFromNowDiscardsParentSnapshotObservedBeforeBinding(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 18, 30, 0, 0, time.UTC)
	sessionKey := "room:group:parent-late-from-now"
	scopeID := "root-parent-late-from-now"
	goalID := "goal-parent-late-from-now"
	createUsageSourceTestGoal(t, repository, goalID, sessionKey, 0, now)
	if _, err := repository.BindUsageScopeFromNow(ctx, protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goalID,
		BoundAt:        now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	late := parentUsageSnapshot(
		"owner-a",
		sessionKey,
		scopeID,
		"slot-parent-late",
		protocol.GoalUsage{
			InputTokens:       8,
			OutputTokens:      2,
			ActualTotalTokens: 12,
		},
		true,
		now.Add(time.Minute),
	)
	result, err := repository.RecordUsageParentSnapshot(ctx, late)
	if err != nil {
		t.Fatal(err)
	}
	if result.Goal != nil || result.AttributedUsage.ActualTokens() != 0 {
		t.Fatalf("late pre-bind parent result = %#v, want discarded", result)
	}
	stored, err := repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Usage.ActualTokens() != 0 {
		t.Fatalf("stored Goal usage = %#v, want zero", stored.Usage)
	}
	var discarded bool
	if err := repository.db.QueryRow(
		`SELECT discarded
		 FROM goal_usage_parent_ledger
		 WHERE owner_user_id = ? AND goal_session_key = ?
		   AND scope_round_id = ? AND source_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
		"slot-parent-late",
	).Scan(&discarded); err != nil {
		t.Fatal(err)
	}
	if !discarded {
		t.Fatal("late pre-bind parent ledger was not tombstoned")
	}
}

func TestRepositoryParentUsagePresenceControlsFinalization(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tokenObserved bool
		wantBlocked   bool
	}{
		{name: "explicit zero is authoritative", tokenObserved: true},
		{name: "missing provider usage remains unavailable", wantBlocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := newTestRepository(t)
			ctx := context.Background()
			now := time.Date(2026, 7, 27, 18, 30, 0, 0, time.UTC)
			sessionKey := "room:group:parent-presence-" + tc.name
			scopeID := "root-parent-presence"
			goal := scopeTestGoal("goal-parent-presence", sessionKey, now.Add(time.Minute))

			snapshot := parentUsageSnapshot(
				"owner-a",
				sessionKey,
				scopeID,
				"slot-terminal",
				protocol.GoalUsage{},
				tc.tokenObserved,
				now,
			)
			if _, err := repository.RecordUsageParentSnapshot(ctx, snapshot); err != nil {
				t.Fatal(err)
			}
			binding := protocol.GoalUsageScopeBinding{
				OwnerUserID:    "owner-a",
				GoalSessionKey: sessionKey,
				SourceKind:     protocol.GoalUsageSourceKindNXSTask,
				ScopeRoundID:   scopeID,
				GoalID:         goal.ID,
				BoundAt:        goal.CreatedAt,
				UsageEventID:   "event-parent-presence-usage",
			}
			created, err := repository.CreateGoalWithUsageScope(
				ctx,
				goal,
				scopeTestCreatedEvent(goal, "event-parent-presence-created", scopeID),
				binding,
			)
			if err != nil {
				t.Fatal(err)
			}
			if created.TokenUsageUnavailable != tc.wantBlocked {
				t.Fatalf("create unavailable = %v, want %v", created.TokenUsageUnavailable, tc.wantBlocked)
			}
			current := created.Goal
			if current == nil {
				t.Fatal("atomic create returned nil Goal")
			}
			completedAt := now.Add(2 * time.Minute)
			current.Status = protocol.GoalStatusComplete
			current.CompletedAt = &completedAt
			current.UpdatedAt = completedAt
			current.Version++
			current, err = repository.UpdateGoal(ctx, *current, current.Version-1)
			if err != nil {
				t.Fatal(err)
			}
			finalizedAt := now.Add(3 * time.Minute)
			current.UsageFinalized = true
			current.UsageFinalizedAt = &finalizedAt
			current.UpdatedAt = finalizedAt
			current.Version++
			event := protocol.GoalEvent{
				ID:         "event-parent-presence-finalized",
				GoalID:     current.ID,
				SessionKey: current.SessionKey,
				EventType:  "usage_finalized",
				Source:     protocol.GoalUpdateSourceSystem,
				CreatedAt:  finalizedAt,
			}
			finalized, finalizeErr := repository.FinalizeGoalUsage(
				ctx,
				*current,
				current.Version-1,
				event,
			)
			if tc.wantBlocked {
				if !errors.Is(finalizeErr, ErrGoalUsageUnavailable) {
					t.Fatalf("FinalizeGoalUsage() error = %v, want ErrGoalUsageUnavailable", finalizeErr)
				}
				if finalized != nil {
					t.Fatalf("blocked finalization Goal = %#v, want nil", finalized)
				}
				return
			}
			if finalizeErr != nil {
				t.Fatal(finalizeErr)
			}
			if finalized == nil || !finalized.UsageFinalized {
				t.Fatalf("explicit-zero finalized Goal = %#v", finalized)
			}
		})
	}
}

func parentUsageSnapshot(
	ownerUserID string,
	goalSessionKey string,
	scopeRoundID string,
	sourceRoundID string,
	usage protocol.GoalUsage,
	tokenUsageObserved bool,
	observedAt time.Time,
) protocol.GoalUsageParentSnapshot {
	return protocol.GoalUsageParentSnapshot{
		OwnerUserID:        ownerUserID,
		GoalSessionKey:     goalSessionKey,
		ScopeRoundID:       scopeRoundID,
		SourceRoundID:      sourceRoundID,
		Usage:              usage,
		TokenUsageObserved: tokenUsageObserved,
		ObservedAt:         observedAt,
	}
}
