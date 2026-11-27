// usage_test.go 覆盖 Goal 用量证据、父账本、记账、作用域与来源测试。
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

func TestRepositoryRecordGoalUsageRollsBackAggregateWhenEventFails(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 30, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-usage-rollback",
		SessionKey: "agent:nexus:ws:dm:usage-rollback",
		Objective:  "retry without duplication",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := protocol.GoalEvent{
		ID:         "event-usage-duplicate",
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  now,
	}
	if err := repository.AppendEvent(ctx, duplicate); err != nil {
		t.Fatal(err)
	}

	next := *created
	next.Usage = protocol.GoalUsage{
		InputTokens:       10,
		OutputTokens:      2,
		ActualTotalTokens: 42,
		ActualTotalKnown:  true,
	}.NormalizeTotals()
	next.Version++
	next.UpdatedAt = now.Add(time.Second)
	duplicate.EventType = "usage_recorded"
	if _, err := repository.RecordGoalUsage(
		ctx,
		next,
		created.Version,
		[]protocol.GoalEvent{duplicate},
	); err == nil {
		t.Fatal("RecordGoalUsage() error = nil, want duplicate event failure")
	}

	stored, err := repository.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Usage.ActualTokens() != 0 || stored.Usage.BudgetTokens() != 0 ||
		stored.Version != created.Version {
		t.Fatalf("stored = %#v, want aggregate update rolled back with event", stored)
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
