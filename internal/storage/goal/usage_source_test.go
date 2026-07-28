package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryApplyUsageSourceSnapshotPersistsExplicitZero(t *testing.T) {
	repository := newTestRepository(t)
	now := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)

	result, err := repository.ApplyUsageSourceSnapshot(
		context.Background(),
		usageSourceTestSnapshot("task-zero", 0, "", "", "", now),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != (protocol.GoalUsageSourceResult{}) {
		t.Fatalf("zero result = %#v, want no token delta", result)
	}
	assertUsageSourceCheckpoint(t, repository.db, "task-zero", 0, "", "")
}

func TestRepositoryApplyUsageSourceSnapshotAttributesOnlyNewHighWater(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	createUsageSourceTestGoal(t, repository, "goal-source", "agent:nexus:ws:dm:source", 10, now)

	first, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-1",
		100,
		"event-source-1",
		"goal-source",
		"agent:nexus:ws:dm:source",
		now.Add(time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if first.ObservedDelta != 100 || first.AttributedDelta != 100 ||
		first.Goal == nil || first.Goal.Usage.ActualTokens() != 110 || first.Goal.Version != 2 ||
		first.Event == nil || first.Event.EventType != "usage_recorded" {
		t.Fatalf("first result = %#v, want 100-token attribution and goal actual=110 v2", first)
	}
	eventUsage, ok := first.Event.Payload["usage"].(protocol.GoalUsage)
	if !ok || eventUsage.ActualTokens() != 100 || eventUsage.BudgetTokens() != 0 {
		t.Fatalf("first event usage = %#v, want actual=100 budget=0", first.Event.Payload["usage"])
	}

	second, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-1",
		150,
		"event-source-2",
		"goal-source",
		"agent:nexus:ws:dm:source",
		now.Add(2*time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if second.ObservedDelta != 50 || second.AttributedDelta != 50 ||
		second.Goal == nil || second.Goal.Usage.ActualTokens() != 160 || second.Goal.Version != 3 {
		t.Fatalf("second result = %#v, want 50-token attribution and goal actual=160 v3", second)
	}

	for _, snapshot := range []protocol.GoalUsageSourceSnapshot{
		usageSourceTestSnapshot(
			"task-1",
			150,
			"event-source-duplicate",
			"goal-source",
			"agent:nexus:ws:dm:source",
			now.Add(3*time.Minute),
		),
		usageSourceTestSnapshot(
			"task-1",
			120,
			"event-source-out-of-order",
			"goal-source",
			"agent:nexus:ws:dm:source",
			now.Add(4*time.Minute),
		),
	} {
		result, applyErr := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
		if applyErr != nil {
			t.Fatal(applyErr)
		}
		if result.ObservedDelta != 0 || result.AttributedDelta != 0 || result.Goal != nil || result.Event != nil {
			t.Fatalf("duplicate/out-of-order result = %#v, want empty zero delta", result)
		}
	}

	stored, err := repository.GetGoal(ctx, "goal-source")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 160 || stored.Version != 3 {
		t.Fatalf("stored goal = %#v, want actual=160 v3", stored)
	}
	events, err := repository.ListEvents(ctx, "goal-source", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v, want exactly two attributed deltas", events)
	}
	assertUsageSourceCheckpoint(t, repository.db, "task-1", 150, "goal-source", "round-source")
}

func TestRepositoryApplyUsageSourceSnapshotWithoutGoalAdvancesOnlyCheckpoint(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 11, 0, 0, 0, time.UTC)
	createUsageSourceTestGoal(t, repository, "goal-later", "agent:nexus:ws:dm:later", 0, now)

	observed, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-later",
		40,
		"",
		"",
		"",
		now.Add(time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if observed.ObservedDelta != 40 || observed.AttributedDelta != 0 ||
		observed.Goal != nil || observed.Event != nil {
		t.Fatalf("observe-only result = %#v, want observed=40 and no attribution", observed)
	}

	attributed, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-later",
		70,
		"event-later",
		"goal-later",
		"agent:nexus:ws:dm:later",
		now.Add(2*time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if attributed.ObservedDelta != 30 || attributed.AttributedDelta != 30 ||
		attributed.Goal == nil || attributed.Goal.Usage.ActualTokens() != 30 {
		t.Fatalf("later attribution = %#v, want only post-checkpoint delta 30", attributed)
	}
	assertUsageSourceCheckpoint(t, repository.db, "task-later", 70, "goal-later", "round-source")
}

func TestRepositoryClaimUsageSourceRoundBackfillsPendingExactlyOnceWithoutCrossRoundAttribution(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 11, 30, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:usage-source"
	nextGoalSessionKey := "agent:nexus:ws:dm:next-goal-session"
	createUsageSourceTestGoal(t, repository, "goal-round-create", sessionKey, 0, now)
	createUsageSourceTestGoal(t, repository, "goal-next-round", nextGoalSessionKey, 0, now)

	for _, input := range []struct {
		sourceID   string
		cumulative int64
	}{
		{sourceID: "task-round-a", cumulative: 40},
		{sourceID: "task-round-b", cumulative: 60},
	} {
		snapshot := usageSourceTestSnapshot(
			input.sourceID,
			input.cumulative,
			"",
			"",
			"",
			now.Add(time.Minute),
		)
		snapshot.RoundID = "round-create"
		result, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if result.ObservedDelta != input.cumulative || result.AttributedDelta != 0 {
			t.Fatalf("observe-only result = %#v, want pending delta %d", result, input.cumulative)
		}
		assertUsageSourcePending(t, repository.db, input.sourceID, "round-create", input.cumulative)
	}

	claim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:       "owner-source",
		RuntimeSessionKey: sessionKey,
		SourceKind:        protocol.GoalUsageSourceKindNXSTask,
		RoundID:           "round-create",
		GoalID:            "goal-round-create",
		GoalSessionKey:    sessionKey,
		EventID:           "event-round-create-claim",
		ClaimedAt:         now.Add(2 * time.Minute),
	}
	claimed, err := repository.ClaimUsageSourceRound(ctx, claim)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ObservedDelta != 0 || claimed.AttributedDelta != 100 ||
		claimed.Goal == nil || claimed.Goal.Usage.ActualTokens() != 100 ||
		claimed.Event == nil || claimed.Event.RoundID != "round-create" {
		t.Fatalf("claim result = %#v, want one 100-token round-start attribution", claimed)
	}
	for _, sourceID := range []string{"task-round-a", "task-round-b"} {
		assertUsageSourcePending(t, repository.db, sourceID, "", 0)
	}

	duplicate, err := repository.ClaimUsageSourceRound(ctx, claim)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.AttributedDelta != 0 || duplicate.Goal != nil || duplicate.Event != nil {
		t.Fatalf("duplicate claim = %#v, want idempotent no-op", duplicate)
	}

	nextSnapshot := usageSourceTestSnapshot(
		"task-round-a",
		70,
		"",
		"",
		"",
		now.Add(3*time.Minute),
	)
	nextSnapshot.RoundID = "round-next"
	nextSnapshot.GoalSessionKey = nextGoalSessionKey
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, nextSnapshot); err != nil {
		t.Fatal(err)
	}
	assertUsageSourcePending(t, repository.db, "task-round-a", "round-next", 30)

	wrongRound := claim
	wrongRound.GoalID = "goal-next-round"
	wrongRound.GoalSessionKey = nextGoalSessionKey
	wrongRound.EventID = "event-wrong-round"
	wrongRound.ClaimedAt = now.Add(4 * time.Minute)
	wrongRoundResult, err := repository.ClaimUsageSourceRound(ctx, wrongRound)
	if err != nil {
		t.Fatal(err)
	}
	if wrongRoundResult.AttributedDelta != 0 {
		t.Fatalf("old-round claim = %#v, want no cross-round attribution", wrongRoundResult)
	}
	nextGoal, err := repository.GetGoal(ctx, "goal-next-round")
	if err != nil {
		t.Fatal(err)
	}
	if nextGoal == nil || nextGoal.Usage.ActualTokens() != 0 {
		t.Fatalf("next goal after wrong-round claim = %#v, want zero usage", nextGoal)
	}

	nextClaim := wrongRound
	nextClaim.RoundID = "round-next"
	nextClaim.EventID = "event-next-round"
	nextClaim.ClaimedAt = now.Add(5 * time.Minute)
	nextResult, err := repository.ClaimUsageSourceRound(ctx, nextClaim)
	if err != nil {
		t.Fatal(err)
	}
	if nextResult.AttributedDelta != 30 ||
		nextResult.Goal == nil || nextResult.Goal.Usage.ActualTokens() != 30 {
		t.Fatalf("next-round claim = %#v, want only new-round delta 30", nextResult)
	}
}

func TestRepositoryApplyUsageSourceSnapshotRejectsFinalizedGoalWithoutAdvancingCheckpoint(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 11, 45, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:usage-source"
	createFinalizedUsageSourceTestGoal(t, repository, "goal-finalized-snapshot", sessionKey, 10, now)

	observed := usageSourceTestSnapshot(
		"task-finalized-snapshot",
		40,
		"",
		"",
		"",
		now.Add(time.Minute),
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, observed); err != nil {
		t.Fatal(err)
	}
	assertUsageSourceCheckpoint(t, repository.db, "task-finalized-snapshot", 40, "", "")
	assertUsageSourcePending(t, repository.db, "task-finalized-snapshot", "round-source", 40)

	attributed := usageSourceTestSnapshot(
		"task-finalized-snapshot",
		70,
		"event-finalized-snapshot",
		"goal-finalized-snapshot",
		sessionKey,
		now.Add(2*time.Minute),
	)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, attributed); !errors.Is(err, errGoalUsageSourceFinalized) {
		t.Fatalf("finalized snapshot error = %v, want errGoalUsageSourceFinalized", err)
	}

	assertUsageSourceCheckpoint(t, repository.db, "task-finalized-snapshot", 40, "", "")
	assertUsageSourcePending(t, repository.db, "task-finalized-snapshot", "round-source", 40)
	stored, err := repository.GetGoal(ctx, "goal-finalized-snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || !stored.UsageFinalized ||
		stored.Usage.ActualTokens() != 10 || stored.Version != 1 {
		t.Fatalf("finalized goal after rejected snapshot = %#v, want actual=10 v1 unchanged", stored)
	}
	events, err := repository.ListEvents(ctx, "goal-finalized-snapshot", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events after rejected snapshot = %#v, want none", events)
	}
}

func TestRepositoryClaimUsageSourceRoundRejectsFinalizedGoalWithoutClearingPending(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 11, 50, 0, 0, time.UTC)
	sessionKey := "agent:nexus:ws:dm:usage-source"
	createFinalizedUsageSourceTestGoal(t, repository, "goal-finalized-claim", sessionKey, 5, now)

	observed := usageSourceTestSnapshot(
		"task-finalized-claim",
		80,
		"",
		"",
		"",
		now.Add(time.Minute),
	)
	observed.RoundID = "round-finalized-claim"
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, observed); err != nil {
		t.Fatal(err)
	}
	assertUsageSourceCheckpoint(t, repository.db, "task-finalized-claim", 80, "", "")
	assertUsageSourcePending(t, repository.db, "task-finalized-claim", "round-finalized-claim", 80)

	claim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:       "owner-source",
		RuntimeSessionKey: sessionKey,
		SourceKind:        protocol.GoalUsageSourceKindNXSTask,
		RoundID:           "round-finalized-claim",
		GoalID:            "goal-finalized-claim",
		GoalSessionKey:    sessionKey,
		EventID:           "event-finalized-claim",
		ClaimedAt:         now.Add(2 * time.Minute),
	}
	if _, err := repository.ClaimUsageSourceRound(ctx, claim); !errors.Is(err, errGoalUsageSourceFinalized) {
		t.Fatalf("finalized claim error = %v, want errGoalUsageSourceFinalized", err)
	}

	assertUsageSourceCheckpoint(t, repository.db, "task-finalized-claim", 80, "", "")
	assertUsageSourcePending(t, repository.db, "task-finalized-claim", "round-finalized-claim", 80)
	stored, err := repository.GetGoal(ctx, "goal-finalized-claim")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || !stored.UsageFinalized ||
		stored.Usage.ActualTokens() != 5 || stored.Version != 1 {
		t.Fatalf("finalized goal after rejected claim = %#v, want actual=5 v1 unchanged", stored)
	}
	events, err := repository.ListEvents(ctx, "goal-finalized-claim", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events after rejected claim = %#v, want none", events)
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

func TestRepositoryApplyUsageSourceSnapshotEventFailureRollsBackGoalAndCheckpoint(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	createUsageSourceTestGoal(t, repository, "goal-event", "agent:nexus:ws:dm:event", 0, now)

	_, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-event-a",
		20,
		"event-shared",
		"goal-event",
		"agent:nexus:ws:dm:event",
		now.Add(time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-event-b",
		30,
		"event-shared",
		"goal-event",
		"agent:nexus:ws:dm:event",
		now.Add(2*time.Minute),
	))
	if err == nil {
		t.Fatal("duplicate event id error = nil, want transaction failure")
	}

	stored, err := repository.GetGoal(ctx, "goal-event")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 20 || stored.Version != 2 {
		t.Fatalf("goal after rolled-back event = %#v, want actual=20 v2", stored)
	}
	assertUsageSourceCheckpointMissing(t, repository.db, "task-event-b")

	retried, err := repository.ApplyUsageSourceSnapshot(ctx, usageSourceTestSnapshot(
		"task-event-b",
		30,
		"event-retry",
		"goal-event",
		"agent:nexus:ws:dm:event",
		now.Add(3*time.Minute),
	))
	if err != nil {
		t.Fatal(err)
	}
	if retried.ObservedDelta != 30 || retried.AttributedDelta != 30 ||
		retried.Goal == nil || retried.Goal.Usage.ActualTokens() != 50 || retried.Goal.Version != 3 {
		t.Fatalf("retry result = %#v, want full 30-token retry and actual=50 v3", retried)
	}
}

func TestRepositoryApplyUsageSourceSnapshotSerializesRoomSourcesIntoSharedGoal(t *testing.T) {
	repository := newTestRepository(t)
	repository.db.SetMaxOpenConns(1)
	repository.db.SetMaxIdleConns(1)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 14, 0, 0, 0, time.UTC)
	createUsageSourceTestGoal(t, repository, "goal-room", "room:group:conversation-source", 0, now)

	snapshots := []protocol.GoalUsageSourceSnapshot{
		usageSourceTestSnapshotForRuntime(
			"agent:a:ws:group:conversation-source",
			"task-room",
			100,
			"event-room-a",
			"goal-room",
			"room:group:conversation-source",
			now.Add(time.Minute),
		),
		usageSourceTestSnapshotForRuntime(
			"agent:b:ws:group:conversation-source",
			"task-room",
			200,
			"event-room-b",
			"goal-room",
			"room:group:conversation-source",
			now.Add(2*time.Minute),
		),
	}
	errs := make(chan error, len(snapshots))
	var waitGroup sync.WaitGroup
	for _, snapshot := range snapshots {
		waitGroup.Add(1)
		go func(current protocol.GoalUsageSourceSnapshot) {
			defer waitGroup.Done()
			result, applyErr := repository.ApplyUsageSourceSnapshot(ctx, current)
			if applyErr == nil && (result.ObservedDelta != current.CumulativeActualTokens ||
				result.AttributedDelta != current.CumulativeActualTokens) {
				applyErr = fmt.Errorf(
					"usage source delta = %d/%d, want %d",
					result.ObservedDelta,
					result.AttributedDelta,
					current.CumulativeActualTokens,
				)
			}
			errs <- applyErr
		}(snapshot)
	}
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	stored, err := repository.GetGoal(ctx, "goal-room")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 300 || stored.Version != 3 {
		t.Fatalf("shared Room goal = %#v, want actual=300 v3", stored)
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

func createFinalizedUsageSourceTestGoal(
	t *testing.T,
	repository *Repository,
	goalID string,
	sessionKey string,
	actualTokens int64,
	now time.Time,
) {
	t.Helper()
	finalizedAt := now.UTC()
	_, err := repository.CreateGoal(context.Background(), protocol.Goal{
		ID:         goalID,
		SessionKey: sessionKey,
		Objective:  "ship finalized source accounting",
		Status:     protocol.GoalStatusComplete,
		Usage: protocol.GoalUsage{
			ActualTotalTokens: actualTokens,
			ActualTotalKnown:  true,
			BudgetTotalKnown:  true,
		},
		UsageFinalized:   true,
		UsageFinalizedAt: &finalizedAt,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
		CompletedAt:      &finalizedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertUsageSourceCheckpoint(
	t *testing.T,
	db *sql.DB,
	sourceID string,
	wantCumulative int64,
	wantGoalID string,
	wantRoundID string,
) {
	t.Helper()
	var cumulative int64
	var goalID, roundID sql.NullString
	err := db.QueryRow(
		`SELECT cumulative_actual_tokens, last_attributed_goal_id, last_attributed_round_id
		 FROM goal_usage_source_checkpoints
		 WHERE owner_user_id = ?
		   AND runtime_session_key = ?
		   AND source_kind = ?
		   AND source_id = ?`,
		"owner-source",
		"agent:nexus:ws:dm:usage-source",
		protocol.GoalUsageSourceKindNXSTask,
		sourceID,
	).Scan(&cumulative, &goalID, &roundID)
	if err != nil {
		t.Fatal(err)
	}
	if cumulative != wantCumulative || goalID.String != wantGoalID || roundID.String != wantRoundID {
		t.Fatalf(
			"checkpoint = cumulative:%d goal:%q round:%q, want %d/%q/%q",
			cumulative,
			goalID.String,
			roundID.String,
			wantCumulative,
			wantGoalID,
			wantRoundID,
		)
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

func assertUsageSourcePending(
	t *testing.T,
	db *sql.DB,
	sourceID string,
	wantRoundID string,
	wantTokens int64,
) {
	t.Helper()
	var count, tokens int64
	var roundID string
	err := db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(pending_actual_tokens), 0), COALESCE(MAX(scope_round_id), '')
		 FROM goal_usage_source_pending
		 WHERE owner_user_id = ?
		   AND runtime_session_key = ?
		   AND source_kind = ?
		   AND source_id = ?`,
		"owner-source",
		"agent:nexus:ws:dm:usage-source",
		protocol.GoalUsageSourceKindNXSTask,
		sourceID,
	).Scan(&count, &tokens, &roundID)
	if err != nil {
		t.Fatal(err)
	}
	wantCount := int64(0)
	if wantTokens > 0 {
		wantCount = 1
	}
	if count != wantCount || roundID != wantRoundID || tokens != wantTokens {
		t.Fatalf(
			"pending checkpoint for %q = count:%d round:%q tokens:%d, want %d/%q/%d",
			sourceID,
			count,
			roundID,
			tokens,
			wantCount,
			wantRoundID,
			wantTokens,
		)
	}
}
