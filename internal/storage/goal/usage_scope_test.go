package goal

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryUsageScopeClaimsAcrossRuntimeAndIsolatesIdentity(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC)
	sessionA := "room:group:scope-a"
	sessionB := "room:group:scope-b"
	createUsageSourceTestGoal(t, repository, "goal-scope-a", sessionA, 0, now)

	snapshots := []protocol.GoalUsageSourceSnapshot{
		scopeTestSnapshot("owner-a", "agent:a:room", "task-a", 10, sessionA, "slot-a", "root-a", now),
		scopeTestSnapshot("owner-a", "agent:b:room", "task-b", 20, sessionA, "slot-b", "root-a", now),
		scopeTestSnapshot("owner-b", "agent:a:room", "task-owner-b", 40, sessionA, "slot-a", "root-a", now),
		scopeTestSnapshot("owner-a", "agent:a:room", "task-session-b", 50, sessionB, "slot-a", "root-a", now),
		scopeTestSnapshot("owner-a", "agent:a:room", "task-scope-b", 60, sessionA, "slot-a", "root-b", now),
	}
	for _, snapshot := range snapshots {
		result, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if result.AttributedDelta != 0 {
			t.Fatalf("pending snapshot result = %#v, want no attribution", result)
		}
	}

	claimed, err := repository.ClaimUsageSourceRound(ctx, scopeTestClaim(
		"owner-a",
		sessionA,
		"root-a",
		"goal-scope-a",
		"event-scope-a",
		now.Add(time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.AttributedDelta != 30 || claimed.Goal == nil || claimed.Goal.Usage.ActualTokens() != 30 {
		t.Fatalf("cross-runtime claim = %#v, want exactly 30 tokens", claimed)
	}

	var pendingTokens int64
	if err := repository.db.QueryRow(
		`SELECT COALESCE(SUM(pending_actual_tokens), 0) FROM goal_usage_source_pending`,
	).Scan(&pendingTokens); err != nil {
		t.Fatal(err)
	}
	if pendingTokens != 150 {
		t.Fatalf("isolated pending tokens = %d, want 150", pendingTokens)
	}
}

func TestRepositoryUsageScopeKeepsSameSourceAcrossScopesAndRoutesAfterBinding(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 13, 30, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:multi-scope"
	createUsageSourceTestGoal(t, repository, "goal-multi-scope", sessionKey, 0, now)

	first := scopeTestSnapshot(
		"owner-a",
		"agent:nexus:ws:dm:multi-scope",
		"task-shared",
		40,
		sessionKey,
		"child-round-a",
		"scope-a",
		now.Add(time.Minute),
	)
	second := first
	second.CumulativeActualTokens = 70
	second.RoundID = "child-round-b"
	second.ScopeRoundID = "scope-b"
	second.ObservedAt = now.Add(2 * time.Minute)
	for _, snapshot := range []protocol.GoalUsageSourceSnapshot{first, second} {
		if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := repository.db.Query(
		`SELECT scope_round_id, pending_actual_tokens
		 FROM goal_usage_source_pending
		 WHERE owner_user_id = ? AND source_id = ?
		 ORDER BY scope_round_id`,
		"owner-a",
		"task-shared",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make(map[string]int64)
	for rows.Next() {
		var scope string
		var tokens int64
		if err := rows.Scan(&scope, &tokens); err != nil {
			t.Fatal(err)
		}
		got[scope] = tokens
	}
	if got["scope-a"] != 40 || got["scope-b"] != 30 || len(got) != 2 {
		t.Fatalf("pending by scope = %#v, want scope-a=40 scope-b=30", got)
	}

	for index, scope := range []string{"scope-a", "scope-b"} {
		result, claimErr := repository.ClaimUsageSourceRound(ctx, scopeTestClaim(
			"owner-a",
			sessionKey,
			scope,
			"goal-multi-scope",
			"event-claim-"+scope,
			now.Add(time.Duration(index+3)*time.Minute),
		))
		if claimErr != nil {
			t.Fatal(claimErr)
		}
		want := int64(40)
		if scope == "scope-b" {
			want = 30
		}
		if result.AttributedDelta != want {
			t.Fatalf("claim %s = %#v, want delta %d", scope, result, want)
		}
	}

	direct := second
	direct.CumulativeActualTokens = 100
	direct.EventID = "event-direct-after-binding"
	direct.ObservedAt = now.Add(6 * time.Minute)
	result, err := repository.ApplyUsageSourceSnapshot(ctx, direct)
	if err != nil {
		t.Fatal(err)
	}
	if result.ObservedDelta != 30 || result.AttributedDelta != 30 ||
		result.Goal == nil || result.Goal.Usage.ActualTokens() != 100 {
		t.Fatalf("bound snapshot = %#v, want direct 30-token attribution and total 100", result)
	}
}

func TestRepositoryExplicitGoalBindingDiscardsExistingScopePending(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 13, 45, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:explicit-drain"
	scopeID := "scope-explicit-drain"
	createUsageSourceTestGoal(t, repository, "goal-explicit-drain", sessionKey, 0, now)

	snapshot := scopeTestSnapshot(
		"owner-a",
		sessionKey,
		"task-explicit-drain",
		40,
		sessionKey,
		"child-explicit-drain",
		scopeID,
		now.Add(time.Minute),
	)
	snapshot.EvidenceRequired = true
	snapshot.Terminal = true
	snapshot.TokenUsageObserved = true
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.CumulativeActualTokens = 70
	snapshot.GoalID = "goal-explicit-drain"
	snapshot.EventID = "event-explicit-drain"
	snapshot.ObservedAt = now.Add(2 * time.Minute)
	result, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.ObservedDelta != 30 || result.AttributedDelta != 30 ||
		result.Goal == nil || result.Goal.Usage.ActualTokens() != 30 {
		t.Fatalf("explicit binding result = %#v, want only current delta 30", result)
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
		t.Fatalf("pending after explicit binding = %d, want 0", pendingCount)
	}
}

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

func TestRepositoryCreateGoalWithUsageScopeIsAtomicAndReplaySafe(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 14, 30, 0, 0, time.UTC)
	sessionKey := "room:group:atomic-scope"
	scopeID := "room-root-round"

	pending := []protocol.GoalUsageSourceSnapshot{
		scopeTestSnapshot("owner-a", "agent:a:room", "task-a", 30, sessionKey, "slot-a", scopeID, now),
		scopeTestSnapshot("owner-a", "agent:b:room", "task-b", 50, sessionKey, "slot-b", scopeID, now),
	}
	for _, snapshot := range pending {
		if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	goal := scopeTestGoal("goal-atomic-scope", sessionKey, now.Add(time.Minute))
	createdEvent := scopeTestCreatedEvent(goal, "event-atomic-created", scopeID)
	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goal.ID,
		BoundAt:        now.Add(time.Minute),
		UsageEventID:   "event-atomic-usage",
	}
	result, err := repository.CreateGoalWithUsageScope(ctx, goal, createdEvent, binding)
	if err != nil {
		t.Fatal(err)
	}
	if result.AttributedDelta != 80 || result.Goal == nil ||
		result.Goal.Usage.ActualTokens() != 80 || result.Goal.Version != 2 ||
		result.UsageEvent == nil || result.UsageEvent.ID != binding.UsageEventID {
		t.Fatalf("atomic create result = %#v, want 80-token Goal v2 and usage event", result)
	}
	events, err := repository.ListEvents(ctx, goal.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("atomic create events = %#v, want created + usage_recorded", events)
	}

	replay := pending[0]
	replay.EventID = "event-replayed-snapshot"
	replayed, err := repository.ApplyUsageSourceSnapshot(ctx, replay)
	if err != nil {
		t.Fatal(err)
	}
	if replayed != (protocol.GoalUsageSourceResult{}) {
		t.Fatalf("replayed snapshot = %#v, want no-op", replayed)
	}
	stored, err := repository.GetGoal(ctx, goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 80 || stored.Version != 2 {
		t.Fatalf("goal after replay = %#v, want unchanged actual=80 v2", stored)
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
