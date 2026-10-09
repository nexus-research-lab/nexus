package goal

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryUsageScopeRejectsRebindAndOwnerDrift(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:scope-owner"
	createUsageSourceTestGoal(t, repository, "goal-owner-a", sessionKey, 0, now)
	completedAt := now
	if _, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:          "goal-owner-rebind",
		SessionKey:  sessionKey,
		Objective:   "must not steal scope",
		Status:      protocol.GoalStatusComplete,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &completedAt,
	}); err != nil {
		t.Fatal(err)
	}

	first := scopeTestClaim("owner-a", sessionKey, "scope-owner", "goal-owner-a", "", now)
	if _, err := repository.ClaimUsageSourceRound(ctx, first); err != nil {
		t.Fatal(err)
	}

	rebind := first
	rebind.GoalID = "goal-owner-rebind"
	if _, err := repository.ClaimUsageSourceRound(ctx, rebind); !errors.Is(err, ErrGoalUsageScopeConflict) {
		t.Fatalf("scope rebind error = %v, want ErrGoalUsageScopeConflict", err)
	}

	drift := scopeTestSnapshot(
		"owner-b",
		"agent:nexus:ws:dm:scope-owner",
		"task-owner-drift",
		10,
		sessionKey,
		"child-owner-drift",
		"scope-owner-b",
		now.Add(time.Minute),
	)
	drift.GoalID = "goal-owner-a"
	drift.EventID = "event-owner-drift"
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, drift); !errors.Is(err, ErrGoalUsageScopeConflict) {
		t.Fatalf("owner drift error = %v, want ErrGoalUsageScopeConflict", err)
	}

	var checkpointCount int
	if err := repository.db.QueryRow(
		`SELECT COUNT(*) FROM goal_usage_source_checkpoints
		 WHERE owner_user_id = ? AND source_id = ?`,
		"owner-b",
		"task-owner-drift",
	).Scan(&checkpointCount); err != nil {
		t.Fatal(err)
	}
	if checkpointCount != 0 {
		t.Fatalf("owner drift checkpoint count = %d, want rollback", checkpointCount)
	}

	ownerDriftClaim := scopeTestClaim(
		"owner-b",
		sessionKey,
		"scope-owner-b-claim",
		"goal-owner-a",
		"event-owner-b-claim",
		now.Add(2*time.Minute),
	)
	if _, err := repository.ClaimUsageSourceRound(ctx, ownerDriftClaim); !errors.Is(err, ErrGoalUsageScopeConflict) {
		t.Fatalf("owner drift claim error = %v, want ErrGoalUsageScopeConflict", err)
	}
}

func TestRepositoryCreateGoalWithUsageScopeRollsBackEveryRecord(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 0, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:atomic-rollback"
	scopeID := "scope-rollback"
	pending := scopeTestSnapshot(
		"owner-a",
		sessionKey,
		"task-rollback",
		25,
		sessionKey,
		"child-rollback",
		scopeID,
		now,
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, pending); err != nil {
		t.Fatal(err)
	}

	createUsageSourceTestGoal(t, repository, "goal-event-holder", "agent:nexus:ws:dm:event-holder", 0, now)
	if err := repository.AppendEvent(ctx, protocol.GoalEvent{
		ID:         "event-duplicate-created",
		GoalID:     "goal-event-holder",
		SessionKey: "agent:nexus:ws:dm:event-holder",
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceModel,
		CreatedAt:  now,
	}); err != nil {
		t.Fatal(err)
	}

	goal := scopeTestGoal("goal-atomic-rollback", sessionKey, now.Add(time.Minute))
	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goal.ID,
		BoundAt:        now.Add(time.Minute),
		UsageEventID:   "event-rollback-usage",
	}
	if _, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-duplicate-created", scopeID),
		binding,
	); err == nil {
		t.Fatal("atomic create duplicate event error = nil")
	}
	stored, err := repository.GetGoal(ctx, goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored != nil {
		t.Fatalf("rolled-back goal = %#v, want nil", stored)
	}
	var state string
	var goalID sql.NullString
	if err := repository.db.QueryRow(
		`SELECT state, goal_id FROM goal_usage_scope_bindings
		 WHERE owner_user_id = ? AND goal_session_key = ? AND source_kind = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		protocol.GoalUsageSourceKindNXSTask,
		scopeID,
	).Scan(&state, &goalID); err != nil {
		t.Fatal(err)
	}
	if state != "open" || goalID.Valid {
		t.Fatalf("scope after rollback = %q/%v, want open/unbound", state, goalID)
	}
	var pendingTokens int64
	if err := repository.db.QueryRow(
		`SELECT pending_actual_tokens FROM goal_usage_source_pending
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		sessionKey,
		scopeID,
	).Scan(&pendingTokens); err != nil {
		t.Fatal(err)
	}
	if pendingTokens != 25 {
		t.Fatalf("pending after rollback = %d, want 25", pendingTokens)
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

func TestRepositoryFinalizeGoalUsageRejectsBoundPending(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 16, 0, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:pending-finalize"
	scopeID := "scope-pending-finalize"
	completedAt := now.Add(time.Minute)
	_, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:          "goal-pending-finalize",
		SessionKey:  sessionKey,
		Objective:   "settle every child",
		Status:      protocol.GoalStatusComplete,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := scopeTestSnapshot(
		"owner-a",
		sessionKey,
		"task-pending-finalize",
		35,
		sessionKey,
		"child-pending-finalize",
		scopeID,
		now,
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, pending); err != nil {
		t.Fatal(err)
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         "goal-pending-finalize",
		BoundAt:        completedAt,
		UsageEventID:   "event-pending-finalize-claim",
	}
	if err := repository.establishGoalUsageScopeBinding(ctx, tx, binding); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	finalizedAt := now.Add(2 * time.Minute)
	finalGoal := protocol.Goal{
		ID:               "goal-pending-finalize",
		SessionKey:       sessionKey,
		Objective:        "settle every child",
		Status:           protocol.GoalStatusComplete,
		UsageFinalized:   true,
		UsageFinalizedAt: &finalizedAt,
		Version:          2,
		CreatedAt:        now,
		UpdatedAt:        finalizedAt,
		CompletedAt:      &completedAt,
	}
	event := protocol.GoalEvent{
		ID:         "event-finalize-pending",
		GoalID:     finalGoal.ID,
		SessionKey: sessionKey,
		EventType:  "usage_finalized",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  finalizedAt,
	}
	if _, err := repository.FinalizeGoalUsage(ctx, finalGoal, 1, event); !errors.Is(err, ErrGoalUsagePending) {
		t.Fatalf("FinalizeGoalUsage() error = %v, want ErrGoalUsagePending", err)
	}
	stored, err := repository.GetGoal(ctx, finalGoal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.UsageFinalized || stored.Version != 1 {
		t.Fatalf("goal after blocked finalization = %#v, want unfinalized v1", stored)
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
