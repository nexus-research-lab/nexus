// usage_test.go 覆盖 Goal 用量证据、父账本、记账、作用域与来源测试。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryFromNowLateRunningChildKeepsGoalUsageUnavailable(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
	sessionKey := "room:group:child-late-from-now"
	scopeID := "root-child-late-from-now"
	goalID := "goal-child-late-from-now"
	createUsageSourceTestGoal(t, repository, goalID, sessionKey, 0, now)

	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goalID,
		BoundAt:        now.Add(2 * time.Minute),
	}
	if _, err := repository.BindUsageScopeFromNow(ctx, binding); err != nil {
		t.Fatal(err)
	}

	// 这条 progress 在 Bind 前已被 Room 观察，只是持久化晚于 Bind 提交。
	// 它证明 child 在 activation 时已经运行，却不能提供 activation 瞬时基线。
	progress := scopeTestSnapshot(
		"owner-a",
		"agent:child:late",
		"task-late-running",
		30,
		sessionKey,
		"slot-late-running",
		scopeID,
		now.Add(time.Minute),
	)
	progress.EvidenceRequired = true
	progress.EventID = "event-child-late-progress"
	result, err := repository.ApplyUsageSourceSnapshot(ctx, progress)
	if err != nil {
		t.Fatal(err)
	}
	if result.AttributedDelta != 0 {
		t.Fatalf("late pre-bind progress = %#v, want no attribution", result)
	}

	terminal := progress
	terminal.CumulativeActualTokens = 50
	terminal.Terminal = true
	terminal.TokenUsageObserved = true
	terminal.ObservedAt = now.Add(3 * time.Minute)
	result, err = repository.ApplyUsageSourceSnapshot(ctx, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if result.AttributedDelta != 0 || !result.TokenUsageUnavailable {
		t.Fatalf("terminal after unavailable baseline = %#v, want no attribution and unavailable", result)
	}
	stored, err := repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Usage.ActualTokens() != 0 {
		t.Fatalf("stored Goal usage = %#v, want zero without exact child baseline", stored.Usage)
	}
	if finalized, finalizeErr := completeAndFinalizeEvidenceGoal(
		t,
		repository,
		stored,
		now.Add(4*time.Minute),
		"event-child-late-finalized",
	); !errors.Is(finalizeErr, ErrGoalUsageUnavailable) || finalized != nil {
		t.Fatalf("late child finalization = %#v, %v, want unavailable", finalized, finalizeErr)
	}
}

func completeAndFinalizeEvidenceGoal(
	t *testing.T,
	repository *Repository,
	current *protocol.Goal,
	completedAt time.Time,
	eventID string,
) (*protocol.Goal, error) {
	t.Helper()
	ctx := context.Background()
	current.Status = protocol.GoalStatusComplete
	current.CompletedAt = &completedAt
	current.UpdatedAt = completedAt
	current.Version++
	updated, err := repository.UpdateGoal(ctx, *current, current.Version-1)
	if err != nil {
		t.Fatal(err)
	}
	finalizedAt := completedAt.Add(time.Minute)
	updated.UsageFinalized = true
	updated.UsageFinalizedAt = &finalizedAt
	updated.UpdatedAt = finalizedAt
	updated.Version++
	return repository.FinalizeGoalUsage(ctx, *updated, updated.Version-1, protocol.GoalEvent{
		ID:         eventID,
		GoalID:     updated.ID,
		SessionKey: updated.SessionKey,
		EventType:  "usage_finalized",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  finalizedAt,
	})
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

func TestRepositoryDeleteGoalLeavesClosedUsageScopeTombstone(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 30, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:closed-scope"
	scopeID := "scope-closed"
	createUsageSourceTestGoal(t, repository, "goal-closed-scope", sessionKey, 0, now)

	snapshot := scopeTestSnapshot(
		"owner-a",
		sessionKey,
		"task-closed",
		10,
		sessionKey,
		"child-closed",
		scopeID,
		now.Add(time.Minute),
	)
	snapshot.EvidenceRequired = true
	snapshot.Terminal = true
	snapshot.TokenUsageObserved = true
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	claim := scopeTestClaim(
		"owner-a",
		sessionKey,
		scopeID,
		"goal-closed-scope",
		"event-closed-claim",
		now.Add(2*time.Minute),
	)
	if _, err := repository.ClaimUsageSourceRound(ctx, claim); err != nil {
		t.Fatal(err)
	}
	if deleted, err := repository.DeleteGoal(ctx, "goal-closed-scope"); err != nil || !deleted {
		t.Fatalf("DeleteGoal() = %v, %v, want true nil", deleted, err)
	}

	snapshot.CumulativeActualTokens = 30
	snapshot.GoalID = "goal-closed-scope"
	snapshot.EventID = "event-late-closed"
	snapshot.ObservedAt = now.Add(3 * time.Minute)
	late, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if late.ObservedDelta != 20 || late.AttributedDelta != 0 || late.Goal != nil {
		t.Fatalf("late closed snapshot = %#v, want checkpoint-only delta 20", late)
	}
	var state string
	var goalID sql.NullString
	var closedAt sql.NullTime
	if err := repository.db.QueryRow(
		`SELECT state, goal_id, closed_at FROM goal_usage_scope_bindings
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
	).Scan(&state, &goalID, &closedAt); err != nil {
		t.Fatal(err)
	}
	if state != "closed" || goalID.Valid || !closedAt.Valid {
		t.Fatalf("deleted scope = state:%q goal:%v closed:%v, want closed/null/timestamp", state, goalID, closedAt)
	}
	var pendingCount int
	if err := repository.db.QueryRow(
		`SELECT COUNT(*) FROM goal_usage_source_pending
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
	).Scan(&pendingCount); err != nil {
		t.Fatal(err)
	}
	if pendingCount != 0 {
		t.Fatalf("late closed pending count = %d, want 0", pendingCount)
	}
	var evidenceCount int
	if err := repository.db.QueryRow(
		`SELECT COUNT(*) FROM goal_usage_source_evidence
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
	).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if evidenceCount != 0 {
		t.Fatalf("deleted scope evidence count = %d, want 0", evidenceCount)
	}

	createUsageSourceTestGoal(t, repository, "goal-after-closed", sessionKey, 0, now.Add(4*time.Minute))
	rebind := claim
	rebind.GoalID = "goal-after-closed"
	rebind.EventID = "event-rebind-closed"
	if _, err := repository.ClaimUsageSourceRound(ctx, rebind); !errors.Is(err, ErrGoalUsageScopeConflict) {
		t.Fatalf("closed scope rebind error = %v, want ErrGoalUsageScopeConflict", err)
	}
}

func scopeTestSnapshot(
	ownerUserID string,
	runtimeSessionKey string,
	sourceID string,
	cumulative int64,
	goalSessionKey string,
	sourceRoundID string,
	scopeRoundID string,
	observedAt time.Time,
) protocol.GoalUsageSourceSnapshot {
	return protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            ownerUserID,
		RuntimeSessionKey:      runtimeSessionKey,
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               sourceID,
		CumulativeActualTokens: cumulative,
		GoalSessionKey:         goalSessionKey,
		RoundID:                sourceRoundID,
		ScopeRoundID:           scopeRoundID,
		ObservedAt:             observedAt,
	}
}

func scopeTestClaim(
	ownerUserID string,
	goalSessionKey string,
	scopeRoundID string,
	goalID string,
	eventID string,
	claimedAt time.Time,
) protocol.GoalUsageSourceRoundClaim {
	return protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:       ownerUserID,
		RuntimeSessionKey: goalSessionKey,
		SourceKind:        protocol.GoalUsageSourceKindNXSTask,
		RoundID:           scopeRoundID,
		ScopeRoundID:      scopeRoundID,
		GoalID:            goalID,
		GoalSessionKey:    goalSessionKey,
		EventID:           eventID,
		ClaimedAt:         claimedAt,
	}
}

func scopeTestGoal(goalID string, sessionKey string, now time.Time) protocol.Goal {
	return protocol.Goal{
		ID:         goalID,
		SessionKey: sessionKey,
		Objective:  "ship durable usage scope",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedBy:  "model",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func scopeTestCreatedEvent(goal protocol.Goal, eventID string, roundID string) protocol.GoalEvent {
	return protocol.GoalEvent{
		ID:         eventID,
		GoalID:     goal.ID,
		SessionKey: goal.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceModel,
		RoundID:    roundID,
		Payload:    map[string]any{"objective": goal.Objective},
		CreatedAt:  goal.CreatedAt,
	}
}

func TestRepositoryApplyUsageSourceSnapshotSessionMismatchRollsBackCheckpoint(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	createUsageSourceTestGoal(t, repository, "goal-session", "agent:nexus:ws:dm:right", 0, now)

	_, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-session",
		80,
		"event-session-wrong",
		"goal-session",
		"agent:nexus:ws:dm:wrong",
		now.Add(time.Minute),
	))
	if err == nil || !strings.Contains(err.Error(), "session mismatch") {
		t.Fatalf("mismatch error = %v, want session mismatch", err)
	}
	assertUsageSourceCheckpointMissing(t, repository.db, "task-session")

	result, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-session",
		80,
		"event-session-right",
		"goal-session",
		"agent:nexus:ws:dm:right",
		now.Add(2*time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if result.ObservedDelta != 80 || result.AttributedDelta != 80 ||
		result.Goal == nil || result.Goal.Usage.ActualTokens() != 80 {
		t.Fatalf("retry result = %#v, want full 80-token attribution", result)
	}
}

func usageSourceTestSnapshot(
	sourceID string,
	cumulative int64,
	eventID string,
	goalID string,
	goalSessionKey string,
	observedAt time.Time,
) protocol.GoalUsageSourceSnapshot {
	return usageSourceTestSnapshotForRuntime(
		"agent:nexus:ws:dm:usage-source",
		sourceID,
		cumulative,
		eventID,
		goalID,
		goalSessionKey,
		observedAt,
	)
}

func usageSourceTestSnapshotForRuntime(
	runtimeSessionKey string,
	sourceID string,
	cumulative int64,
	eventID string,
	goalID string,
	goalSessionKey string,
	observedAt time.Time,
) protocol.GoalUsageSourceSnapshot {
	if strings.TrimSpace(goalSessionKey) == "" {
		goalSessionKey = runtimeSessionKey
	}
	return protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            "owner-source",
		RuntimeSessionKey:      runtimeSessionKey,
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               sourceID,
		CumulativeActualTokens: cumulative,
		GoalID:                 goalID,
		GoalSessionKey:         goalSessionKey,
		RoundID:                "round-source",
		EventID:                eventID,
		ObservedAt:             observedAt,
	}
}

func createUsageSourceTestGoal(
	t *testing.T,
	repository *Repository,
	goalID string,
	sessionKey string,
	actualTokens int64,
	now time.Time,
) {
	t.Helper()
	_, err := repository.CreateGoal(context.Background(), protocol.Goal{
		ID:         goalID,
		SessionKey: sessionKey,
		Objective:  "ship source accounting",
		Status:     protocol.GoalStatusActive,
		Usage: protocol.GoalUsage{
			ActualTotalTokens: actualTokens,
			ActualTotalKnown:  true,
			BudgetTotalKnown:  true,
		},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertUsageSourceCheckpointMissing(t *testing.T, db *sql.DB, sourceID string) {
	t.Helper()
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*)
		 FROM goal_usage_source_checkpoints
		 WHERE owner_user_id = ?
		   AND runtime_session_key = ?
		   AND source_kind = ?
		   AND source_id = ?`,
		"owner-source",
		"agent:nexus:ws:dm:usage-source",
		protocol.GoalUsageSourceKindNXSTask,
		sourceID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("checkpoint count for %q = %d, want 0", sourceID, count)
	}
}
