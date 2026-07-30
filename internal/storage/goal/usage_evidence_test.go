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

func TestRepositoryMultiChildEvidenceRequiresEveryTerminalTotal(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 19, 20, 0, 0, time.UTC)
	sessionKey := "room:group:multi-child-evidence"
	scopeID := "root-multi-child-evidence"

	good := scopeTestSnapshot(
		"owner-a",
		"agent:child:good",
		"task-good",
		18,
		sessionKey,
		"slot-good",
		scopeID,
		now,
	)
	good.EvidenceRequired = true
	good.Terminal = true
	good.TokenUsageObserved = true
	missing := scopeTestSnapshot(
		"owner-a",
		"agent:child:missing",
		"task-missing",
		0,
		sessionKey,
		"slot-missing",
		scopeID,
		now,
	)
	missing.EvidenceRequired = true
	missing.Terminal = true
	for _, snapshot := range []protocol.GoalUsageSourceSnapshot{good, missing} {
		if _, err := repository.ApplyUsageSourceSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	goal := scopeTestGoal("goal-multi-child-evidence", sessionKey, now.Add(time.Minute))
	created, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-multi-child-created", scopeID),
		protocol.GoalUsageScopeBinding{
			OwnerUserID:    "owner-a",
			GoalSessionKey: sessionKey,
			SourceKind:     protocol.GoalUsageSourceKindNXSTask,
			ScopeRoundID:   scopeID,
			GoalID:         goal.ID,
			BoundAt:        goal.CreatedAt,
			UsageEventID:   "event-multi-child-usage",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.Goal == nil || created.Goal.Usage.ActualTokens() != 18 {
		t.Fatalf("created Goal = %#v, want good child actual 18", created.Goal)
	}
	if finalized, finalizeErr := completeAndFinalizeEvidenceGoal(
		t,
		repository,
		created.Goal,
		now.Add(2*time.Minute),
		"event-multi-child-finalized",
	); !errors.Is(finalizeErr, ErrGoalUsageUnavailable) || finalized != nil {
		t.Fatalf("multi-child finalization = %#v, %v, want unavailable with no finalized Goal", finalized, finalizeErr)
	}
}

func TestRepositoryProgressTokensDoNotReplaceTerminalTokenEvidence(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 19, 25, 0, 0, time.UTC)
	sessionKey := "room:group:progress-is-not-terminal-evidence"
	scopeID := "root-progress-is-not-terminal-evidence"

	progress := scopeTestSnapshot(
		"owner-a",
		"agent:child:progress",
		"task-progress",
		25,
		sessionKey,
		"slot-progress",
		scopeID,
		now,
	)
	progress.EvidenceRequired = true
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, progress); err != nil {
		t.Fatal(err)
	}
	terminalPlaceholder := progress
	terminalPlaceholder.CumulativeActualTokens = 0
	terminalPlaceholder.Terminal = true
	terminalPlaceholder.ObservedAt = now.Add(time.Minute)
	if _, err := repository.ApplyUsageSourceSnapshot(ctx, terminalPlaceholder); err != nil {
		t.Fatal(err)
	}

	goal := scopeTestGoal("goal-progress-terminal-missing", sessionKey, now.Add(2*time.Minute))
	created, err := repository.CreateGoalWithUsageScope(
		ctx,
		goal,
		scopeTestCreatedEvent(goal, "event-progress-terminal-created", scopeID),
		protocol.GoalUsageScopeBinding{
			OwnerUserID:    "owner-a",
			GoalSessionKey: sessionKey,
			SourceKind:     protocol.GoalUsageSourceKindNXSTask,
			ScopeRoundID:   scopeID,
			GoalID:         goal.ID,
			BoundAt:        goal.CreatedAt,
			UsageEventID:   "event-progress-terminal-usage",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.Goal == nil || created.Goal.Usage.ActualTokens() != 25 {
		t.Fatalf("created Goal = %#v, want progress actual 25", created.Goal)
	}
	if finalized, finalizeErr := completeAndFinalizeEvidenceGoal(
		t,
		repository,
		created.Goal,
		now.Add(3*time.Minute),
		"event-progress-terminal-finalized",
	); !errors.Is(finalizeErr, ErrGoalUsageUnavailable) || finalized != nil {
		t.Fatalf("progress-only terminal finalization = %#v, %v, want unavailable", finalized, finalizeErr)
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

func TestRepositoryFromNowChildStartedAfterBindingIsAuthoritative(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 20, 30, 0, 0, time.UTC)
	sessionKey := "room:group:child-post-bind"
	scopeID := "root-child-post-bind"
	goalID := "goal-child-post-bind"
	createUsageSourceTestGoal(t, repository, goalID, sessionKey, 0, now)
	if _, err := repository.BindUsageScopeFromNow(ctx, protocol.GoalUsageScopeBinding{
		OwnerUserID:    "owner-a",
		GoalSessionKey: sessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   scopeID,
		GoalID:         goalID,
		BoundAt:        now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	terminal := scopeTestSnapshot(
		"owner-a",
		"agent:child:new",
		"task-post-bind",
		40,
		sessionKey,
		"slot-post-bind",
		scopeID,
		now.Add(2*time.Minute),
	)
	terminal.EvidenceRequired = true
	terminal.Terminal = true
	terminal.TokenUsageObserved = true
	terminal.EventID = "event-child-post-bind-terminal"
	result, err := repository.ApplyUsageSourceSnapshot(ctx, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if result.AttributedDelta != 40 || result.TokenUsageUnavailable {
		t.Fatalf("post-bind child result = %#v, want authoritative delta 40", result)
	}
	stored, err := repository.GetGoal(ctx, goalID)
	if err != nil {
		t.Fatal(err)
	}
	finalized, err := completeAndFinalizeEvidenceGoal(
		t,
		repository,
		stored,
		now.Add(3*time.Minute),
		"event-child-post-bind-finalized",
	)
	if err != nil || finalized == nil || !finalized.UsageFinalized {
		t.Fatalf("post-bind child finalization = %#v, %v, want finalized", finalized, err)
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
