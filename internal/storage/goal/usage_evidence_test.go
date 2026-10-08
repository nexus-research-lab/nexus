package goal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryChildEvidenceControlsGoalUsageFinalization(t *testing.T) {
	for _, tc := range []struct {
		name         string
		terminal     bool
		cumulative   int64
		wantFinalize error
		wantActual   int64
	}{
		{
			name:         "running child remains pending",
			wantFinalize: ErrGoalUsagePending,
		},
		{
			name:         "terminal placeholder zero is unavailable",
			terminal:     true,
			wantFinalize: ErrGoalUsageUnavailable,
		},
		{
			name:       "terminal positive provider total is authoritative",
			terminal:   true,
			cumulative: 21,
			wantActual: 21,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := newTestRepository(t)
			ctx := context.Background()
			now := time.Date(2026, 7, 27, 19, 0, 0, 0, time.UTC)
			sessionKey := "room:group:child-evidence:" + tc.name
			scopeID := "root-child-evidence"
			goalID := "goal-child-evidence"

			snapshot := scopeTestSnapshot(
				"owner-a",
				"agent:child:evidence",
				"task-evidence",
				tc.cumulative,
				sessionKey,
				"slot-evidence",
				scopeID,
				now,
			)
			snapshot.EvidenceRequired = true
			snapshot.Terminal = tc.terminal
			snapshot.TokenUsageObserved = tc.cumulative > 0
			if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
				t.Fatal(err)
			}
			// 新 repository 实例模拟进程重启；finalization 只能依赖数据库中的
			// evidence，不能依赖 runner 内存 bool。
			repository = NewRepository(config.Config{DatabaseDriver: "sqlite"}, repository.db)

			goal := scopeTestGoal(goalID, sessionKey, now.Add(time.Minute))
			binding := protocol.GoalUsageScopeBinding{
				OwnerUserID:    "owner-a",
				GoalSessionKey: sessionKey,
				SourceKind:     protocol.GoalUsageSourceKindNXSTask,
				ScopeRoundID:   scopeID,
				GoalID:         goal.ID,
				BoundAt:        goal.CreatedAt,
				UsageEventID:   "event-child-evidence-usage",
			}
			created, err := repository.CreateGoalWithUsageScope(
				ctx,
				goal,
				scopeTestCreatedEvent(goal, "event-child-evidence-created", scopeID),
				binding,
			)
			if err != nil {
				t.Fatal(err)
			}
			if created.Goal == nil || created.Goal.Usage.ActualTokens() != tc.wantActual {
				t.Fatalf("created Goal = %#v, want actual %d", created.Goal, tc.wantActual)
			}

			finalized, finalizeErr := completeAndFinalizeEvidenceGoal(
				t,
				repository,
				created.Goal,
				now.Add(2*time.Minute),
				"event-child-evidence-finalized",
			)
			if tc.wantFinalize != nil {
				if !errors.Is(finalizeErr, tc.wantFinalize) {
					t.Fatalf("FinalizeGoalUsage() error = %v, want %v", finalizeErr, tc.wantFinalize)
				}
				if finalized != nil {
					t.Fatalf("blocked finalized Goal = %#v, want nil", finalized)
				}
				return
			}
			if finalizeErr != nil {
				t.Fatal(finalizeErr)
			}
			if finalized == nil || !finalized.UsageFinalized {
				t.Fatalf("finalized Goal = %#v", finalized)
			}
		})
	}
}

func TestRepositoryBindUsageScopeFromNowDiscardsOnlyTerminalChildEvidence(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 19, 30, 0, 0, time.UTC)
	sessionKey := "room:group:child-evidence-from-now"
	scopeID := "root-child-evidence-from-now"
	goalID := "goal-child-evidence-from-now"
	createUsageSourceTestGoal(t, repository, goalID, sessionKey, 0, now)

	terminal := scopeTestSnapshot(
		"owner-a",
		"agent:child:evidence",
		"task-old-terminal",
		10,
		sessionKey,
		"slot-old-terminal",
		scopeID,
		now.Add(time.Minute),
	)
	terminal.EvidenceRequired = true
	terminal.Terminal = true
	terminal.TokenUsageObserved = true
	active := scopeTestSnapshot(
		"owner-a",
		"agent:child:evidence",
		"task-active",
		0,
		sessionKey,
		"slot-active",
		scopeID,
		now.Add(time.Minute),
	)
	active.EvidenceRequired = true
	for _, snapshot := range []protocol.GoalUsageSourceSnapshot{terminal, active} {
		if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goalID,
		BoundAt:        now.Add(2 * time.Minute),
	}
	result, err := repository.BindUsageScopeFromNow(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if result.DiscardedChildPending != 1 || result.DiscardedChildEvidence != 1 {
		t.Fatalf("BindUsageScopeFromNow() = %#v, want one pending and one terminal evidence tombstone", result)
	}

	rows, err := repository.db.Query(
		`SELECT source_id, discarded
		 FROM goal_usage_source_evidence
		 WHERE owner_user_id = ? AND goal_session_key = ? AND scope_round_id = ?
		 ORDER BY source_id`,
		"owner-a",
		sessionKey,
		scopeID,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	discardedByTask := map[string]bool{}
	for rows.Next() {
		var taskID string
		var discarded bool
		if err := rows.Scan(&taskID, &discarded); err != nil {
			t.Fatal(err)
		}
		discardedByTask[taskID] = discarded
	}
	if !discardedByTask["task-old-terminal"] || discardedByTask["task-active"] {
		t.Fatalf("evidence tombstones = %#v, want only old terminal discarded", discardedByTask)
	}

	active.Terminal = true
	active.GoalID = goalID
	active.EventID = "event-active-terminal"
	active.ObservedAt = now.Add(3 * time.Minute)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, active); err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.BindUsageScopeFromNow(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if replayed != (protocol.GoalUsageScopeBindResult{}) {
		t.Fatalf("idempotent BindUsageScopeFromNow() = %#v, want no new discard", replayed)
	}
	var activeDiscarded bool
	if err := repository.db.QueryRow(
		`SELECT discarded FROM goal_usage_source_evidence
		 WHERE owner_user_id = ? AND source_id = ? AND goal_session_key = ? AND scope_round_id = ?`,
		"owner-a",
		"task-active",
		sessionKey,
		scopeID,
	).Scan(&activeDiscarded); err != nil {
		t.Fatal(err)
	}
	if activeDiscarded {
		t.Fatal("binding replay discarded post-binding terminal evidence")
	}
	stored, err := repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	if finalized, finalizeErr := completeAndFinalizeEvidenceGoal(
		t,
		repository,
		stored,
		now.Add(4*time.Minute),
		"event-from-now-active-finalized",
	); !errors.Is(finalizeErr, ErrGoalUsageUnavailable) || finalized != nil {
		t.Fatalf("preserved active child finalization = %#v, %v, want unavailable", finalized, finalizeErr)
	}
}

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
