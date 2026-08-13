// INPUT: Room parent slot 的终态 usage、provider usage presence 与 durable root scope。
// OUTPUT: source-round 幂等 ledger、open scope 暂存、bound Goal exactly-once 归属与 unavailable 证据。
// POS: Room parent usage 跨 handoff/重启回补及最终 precision barrier 的 SQL 边界。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalUsageParentLedgerRow struct {
	usage              protocol.GoalUsage
	tokenUsageObserved bool
	usageAttributed    bool
	discarded          bool
	attributedGoalID   sql.NullString
}

// IsGoalUsageUnavailable 为上层 service 提供不反向依赖 storage 的稳定错误分类。
func (r *Repository) IsGoalUsageUnavailable(err error) bool {
	return errors.Is(err, ErrGoalUsageUnavailable)
}

// RecordUsageParentSnapshot 持久化一个 Room parent slot 的终态 usage。
//
// open scope 只写 ledger；bound scope 会以 source round 为幂等键把该行一次性
// 归入 Goal。provider 完全未返回 usage 时仍保留 evidence，避免把未知误记为零。
func (r *Repository) RecordUsageParentSnapshot(
	ctx context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	snapshot = normalizeGoalUsageParentSnapshot(snapshot)
	if err := validateGoalUsageParentSnapshot(snapshot); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := r.recordUsageParentSnapshotOnce(ctx, snapshot)
		if !errors.Is(err, errGoalUsageScopeRetry) {
			return result, err
		}
	}
	return protocol.GoalUsageParentResult{}, fmt.Errorf("%w after retries", errGoalUsageScopeRetry)
}

func (r *Repository) recordUsageParentSnapshotOnce(
	ctx context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	key := goalUsageScopeKey{
		ownerUserID:    snapshot.OwnerUserID,
		goalSessionKey: snapshot.GoalSessionKey,
		sourceKind:     protocol.GoalUsageSourceKindNXSTask,
		scopeRoundID:   snapshot.ScopeRoundID,
	}
	resolution, err := r.peekGoalUsageScope(ctx, key)
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if resolution.closed {
		locked, lockErr := r.lockGoalUsageScope(ctx, tx, key)
		if lockErr != nil {
			return protocol.GoalUsageParentResult{}, lockErr
		}
		if !locked.closed {
			return protocol.GoalUsageParentResult{}, errGoalUsageScopeRetry
		}
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		return protocol.GoalUsageParentResult{
			TokenUsageUnavailable: parentUsageSnapshotUnavailable(snapshot),
		}, nil
	}

	goalID := strings.TrimSpace(snapshot.GoalID)
	if goalID == "" {
		goalID = strings.TrimSpace(resolution.goalID)
	}
	if goalID == "" {
		locked, lockErr := r.lockGoalUsageScope(ctx, tx, key)
		if lockErr != nil {
			return protocol.GoalUsageParentResult{}, lockErr
		}
		if locked.closed {
			if err := tx.Commit(); err != nil {
				return protocol.GoalUsageParentResult{}, err
			}
			return protocol.GoalUsageParentResult{
				TokenUsageUnavailable: parentUsageSnapshotUnavailable(snapshot),
			}, nil
		}
		if locked.goalID != "" {
			return protocol.GoalUsageParentResult{}, errGoalUsageScopeRetry
		}
		if _, err := r.insertGoalUsageParentLedger(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		return protocol.GoalUsageParentResult{
			TokenUsageUnavailable: parentUsageSnapshotUnavailable(snapshot),
		}, nil
	}

	item, err := r.lockUsageSourceGoal(ctx, tx, goalID)
	if errors.Is(err, sql.ErrNoRows) && resolution.goalID == goalID {
		return protocol.GoalUsageParentResult{}, errGoalUsageScopeRetry
	}
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	if err := validateUsageSourceGoal(*item, snapshot.GoalSessionKey); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	if err := r.validateGoalUsageScopeOwner(ctx, tx, item.ID, snapshot.OwnerUserID); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}

	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    snapshot.OwnerUserID,
		GoalSessionKey: snapshot.GoalSessionKey,
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   snapshot.ScopeRoundID,
		GoalID:         item.ID,
		BoundAt:        snapshot.ObservedAt,
		UsageEventID:   snapshot.EventID,
	}
	if snapshot.GoalID != "" {
		newlyBound, bindErr := r.establishGoalUsageScopeBindingWithStatus(ctx, tx, binding)
		if bindErr != nil {
			return protocol.GoalUsageParentResult{}, bindErr
		}
		if newlyBound {
			// 显式 Goal 代表 external/current-boundary 语义：绑定前 backlog 不属于
			// 这个 Goal。只在首次绑定时清理，幂等重试不能误丢绑定后新证据。
			if _, err := r.discardGoalUsageSourcePending(ctx, tx, binding); err != nil {
				return protocol.GoalUsageParentResult{}, err
			}
			if _, err := r.discardTerminalGoalUsageSourceEvidence(ctx, tx, binding); err != nil {
				return protocol.GoalUsageParentResult{}, err
			}
			if _, err := r.discardGoalUsageParentPending(ctx, tx, binding); err != nil {
				return protocol.GoalUsageParentResult{}, err
			}
		}
	} else {
		locked, lockErr := r.lockExistingGoalUsageScope(ctx, tx, key, goalID)
		if lockErr != nil {
			return protocol.GoalUsageParentResult{}, lockErr
		}
		if !locked {
			return protocol.GoalUsageParentResult{}, errGoalUsageScopeRetry
		}
	}

	if resolution.excludesFromNowSnapshot(snapshot.ObservedAt) {
		if _, err := r.insertGoalUsageParentLedger(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		if err := r.discardGoalUsageParentSnapshot(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		return protocol.GoalUsageParentResult{}, nil
	}

	if _, err := r.insertGoalUsageParentLedger(ctx, tx, snapshot); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	row, err := r.lockGoalUsageParentLedgerRow(ctx, tx, snapshot)
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	if row.discarded {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		return protocol.GoalUsageParentResult{}, nil
	}
	if row.usageAttributed {
		if strings.TrimSpace(row.attributedGoalID.String) != item.ID {
			return protocol.GoalUsageParentResult{}, fmt.Errorf(
				"%w: Room parent round %q is already attributed to Goal %q",
				ErrGoalUsageScopeConflict,
				snapshot.SourceRoundID,
				row.attributedGoalID.String,
			)
		}
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		return protocol.GoalUsageParentResult{
			TokenUsageUnavailable: parentUsageLedgerRowUnavailable(row),
			Goal:                  item,
		}, nil
	}

	if !isStoredGoalUsageZero(row.usage) {
		if snapshot.EventID == "" {
			return protocol.GoalUsageParentResult{}, fmt.Errorf("goal parent usage event id is required")
		}
		if err := r.addUsageSourceGoalUsage(ctx, tx, item, row.usage, snapshot.ObservedAt); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
	}
	if err := r.markGoalUsageParentLedgerAttributed(ctx, tx, snapshot, item.ID, snapshot.ObservedAt); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}

	var event *protocol.GoalEvent
	if !isStoredGoalUsageZero(row.usage) {
		snapshot.TokenUsageObserved = row.tokenUsageObserved
		current := usageParentGoalEvent(snapshot, *item, row.usage)
		if err := r.insertGoalEvent(ctx, tx, current); err != nil {
			return protocol.GoalUsageParentResult{}, err
		}
		event = &current
	}
	updated, err := r.getUsageSourceGoal(ctx, tx, item.ID)
	if err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.GoalUsageParentResult{}, err
	}
	return protocol.GoalUsageParentResult{
		AttributedUsage:       row.usage,
		TokenUsageUnavailable: parentUsageLedgerRowUnavailable(row),
		Goal:                  updated,
		Event:                 event,
	}, nil
}

func normalizeGoalUsageParentSnapshot(
	snapshot protocol.GoalUsageParentSnapshot,
) protocol.GoalUsageParentSnapshot {
	snapshot.OwnerUserID = strings.TrimSpace(snapshot.OwnerUserID)
	snapshot.GoalSessionKey = strings.TrimSpace(snapshot.GoalSessionKey)
	snapshot.ScopeRoundID = strings.TrimSpace(snapshot.ScopeRoundID)
	snapshot.SourceRoundID = strings.TrimSpace(snapshot.SourceRoundID)
	snapshot.GoalID = strings.TrimSpace(snapshot.GoalID)
	snapshot.EventID = strings.TrimSpace(snapshot.EventID)
	snapshot.ObservedAt = snapshot.ObservedAt.UTC()
	snapshot.Usage = snapshot.Usage.NormalizeTotals()
	return snapshot
}

func validateGoalUsageParentSnapshot(snapshot protocol.GoalUsageParentSnapshot) error {
	if snapshot.OwnerUserID == "" ||
		snapshot.GoalSessionKey == "" ||
		snapshot.ScopeRoundID == "" ||
		snapshot.SourceRoundID == "" {
		return fmt.Errorf("goal parent usage snapshot identity is incomplete")
	}
	if !snapshot.TokenUsageObserved && hasStoredGoalUsageTokens(snapshot.Usage) {
		return fmt.Errorf("goal parent usage snapshot has tokens without provider usage evidence")
	}
	return nil
}

func (r *Repository) insertGoalUsageParentLedger(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageParentSnapshot,
) (bool, error) {
	query := fmt.Sprintf(`INSERT INTO goal_usage_parent_ledger (
    owner_user_id,
    goal_session_key,
    scope_round_id,
    source_round_id,
    token_used_input,
    token_used_output,
    token_used_cache_creation,
    token_used_cache_read,
    token_used_reasoning,
    token_used_total,
    token_used_actual_total,
    token_used_actual_estimated,
    runtime_seconds,
    token_usage_observed,
    usage_attributed,
    discarded,
    observed_at
) VALUES (%s)
ON CONFLICT (
    owner_user_id,
    goal_session_key,
    scope_round_id,
    source_round_id
) DO NOTHING`, r.bindList(17))
	result, err := tx.ExecContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.SourceRoundID,
		snapshot.Usage.InputTokens,
		snapshot.Usage.OutputTokens,
		snapshot.Usage.CacheCreationInputTokens,
		snapshot.Usage.CacheReadInputTokens,
		snapshot.Usage.ReasoningTokens,
		snapshot.Usage.BudgetTokens(),
		snapshot.Usage.ActualTokens(),
		snapshot.Usage.ActualTokensAreEstimated(),
		snapshot.Usage.RuntimeSeconds,
		snapshot.TokenUsageObserved,
		false,
		false,
		snapshot.ObservedAt,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *Repository) lockGoalUsageParentLedgerRow(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageParentSnapshot,
) (goalUsageParentLedgerRow, error) {
	query := `SELECT
    token_used_input,
    token_used_output,
    token_used_cache_creation,
    token_used_cache_read,
    token_used_reasoning,
    token_used_total,
    token_used_actual_total,
    token_used_actual_estimated,
    runtime_seconds,
    token_usage_observed,
    usage_attributed,
    discarded,
    attributed_goal_id
FROM goal_usage_parent_ledger
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND scope_round_id = ` + r.bind(3) + `
  AND source_round_id = ` + r.bind(4)
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	var row goalUsageParentLedgerRow
	err := tx.QueryRowContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.SourceRoundID,
	).Scan(
		&row.usage.InputTokens,
		&row.usage.OutputTokens,
		&row.usage.CacheCreationInputTokens,
		&row.usage.CacheReadInputTokens,
		&row.usage.ReasoningTokens,
		&row.usage.BudgetTotalTokens,
		&row.usage.ActualTotalTokens,
		&row.usage.ActualTokensEstimated,
		&row.usage.RuntimeSeconds,
		&row.tokenUsageObserved,
		&row.usageAttributed,
		&row.discarded,
		&row.attributedGoalID,
	)
	row.usage.TotalTokens = row.usage.BudgetTotalTokens
	row.usage.BudgetTotalKnown = true
	normalizeStoredActualTotal(&row.usage)
	return row, err
}

func (r *Repository) markGoalUsageParentLedgerAttributed(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageParentSnapshot,
	goalID string,
	attributedAt time.Time,
) error {
	query := `UPDATE goal_usage_parent_ledger
SET usage_attributed = ` + r.bind(1) + `,
    attributed_goal_id = ` + r.bind(2) + `,
    attributed_at = ` + r.bind(3) + `
WHERE owner_user_id = ` + r.bind(4) + `
  AND goal_session_key = ` + r.bind(5) + `
  AND scope_round_id = ` + r.bind(6) + `
  AND source_round_id = ` + r.bind(7) + `
  AND usage_attributed = ` + r.bind(8) + `
  AND discarded = ` + r.bind(9)
	result, err := tx.ExecContext(
		ctx,
		query,
		true,
		goalID,
		attributedAt,
		snapshot.OwnerUserID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.SourceRoundID,
		false,
		false,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != 1 {
		return fmt.Errorf("goal parent usage attribution affected %d rows, want 1", affected)
	}
	return err
}

func (r *Repository) lockGoalUsageParentPending(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, int64, protocol.GoalUsage, error) {
	query := `SELECT
    token_used_input,
    token_used_output,
    token_used_cache_creation,
    token_used_cache_read,
    token_used_reasoning,
    token_used_total,
    token_used_actual_total,
    token_used_actual_estimated,
    runtime_seconds,
    token_usage_observed
FROM goal_usage_parent_ledger
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND scope_round_id = ` + r.bind(3) + `
  AND usage_attributed = ` + r.bind(4) + `
  AND discarded = ` + r.bind(5) + `
ORDER BY source_round_id`
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	rows, err := tx.QueryContext(
		ctx,
		query,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.ScopeRoundID,
		false,
		false,
	)
	if err != nil {
		return 0, 0, protocol.GoalUsage{}, err
	}
	defer rows.Close()

	var count, unavailable int64
	total := protocol.GoalUsage{}
	for rows.Next() {
		var usage protocol.GoalUsage
		var tokenObserved bool
		if err := rows.Scan(
			&usage.InputTokens,
			&usage.OutputTokens,
			&usage.CacheCreationInputTokens,
			&usage.CacheReadInputTokens,
			&usage.ReasoningTokens,
			&usage.BudgetTotalTokens,
			&usage.ActualTotalTokens,
			&usage.ActualTokensEstimated,
			&usage.RuntimeSeconds,
			&tokenObserved,
		); err != nil {
			return 0, 0, protocol.GoalUsage{}, err
		}
		usage.TotalTokens = usage.BudgetTotalTokens
		usage.BudgetTotalKnown = true
		normalizeStoredActualTotal(&usage)
		total = total.Add(usage)
		count++
		if !tokenObserved {
			unavailable++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, protocol.GoalUsage{}, err
	}
	return count, unavailable, total.NormalizeTotals(), nil
}

func (r *Repository) markGoalUsageParentScopeAttributed(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
	expectedRows int64,
) error {
	query := `UPDATE goal_usage_parent_ledger
SET usage_attributed = ` + r.bind(1) + `,
    attributed_goal_id = ` + r.bind(2) + `,
    attributed_at = ` + r.bind(3) + `
WHERE owner_user_id = ` + r.bind(4) + `
  AND goal_session_key = ` + r.bind(5) + `
  AND scope_round_id = ` + r.bind(6) + `
  AND usage_attributed = ` + r.bind(7) + `
  AND discarded = ` + r.bind(8)
	result, err := tx.ExecContext(
		ctx,
		query,
		true,
		binding.GoalID,
		binding.BoundAt,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.ScopeRoundID,
		false,
		false,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != expectedRows {
		return fmt.Errorf("goal parent usage claim updated %d rows, expected %d", affected, expectedRows)
	}
	return err
}

func (r *Repository) discardGoalUsageParentPending(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, error) {
	query := `UPDATE goal_usage_parent_ledger
SET discarded = ` + r.bind(1) + `
WHERE owner_user_id = ` + r.bind(2) + `
  AND goal_session_key = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4) + `
  AND usage_attributed = ` + r.bind(5) + `
  AND discarded = ` + r.bind(6)
	result, err := tx.ExecContext(
		ctx,
		query,
		true,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.ScopeRoundID,
		false,
		false,
	)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *Repository) discardGoalUsageParentSnapshot(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageParentSnapshot,
) error {
	query := `UPDATE goal_usage_parent_ledger
SET discarded = ` + r.bind(1) + `
WHERE owner_user_id = ` + r.bind(2) + `
  AND goal_session_key = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4) + `
  AND source_round_id = ` + r.bind(5) + `
  AND usage_attributed = ` + r.bind(6)
	_, err := tx.ExecContext(
		ctx,
		query,
		true,
		snapshot.OwnerUserID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.SourceRoundID,
		false,
	)
	return err
}

func (r *Repository) rejectGoalUsageParentFinalization(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
) error {
	query := `SELECT
    l.usage_attributed,
    l.token_usage_observed
FROM goal_usage_parent_ledger l
LEFT JOIN goal_usage_scope_bindings b
  ON b.owner_user_id = l.owner_user_id
 AND b.goal_session_key = l.goal_session_key
 AND b.source_kind = ` + r.bind(1) + `
 AND b.scope_round_id = l.scope_round_id
WHERE l.discarded = ` + r.bind(2) + `
  AND (
       (l.usage_attributed = ` + r.bind(3) + ` AND b.goal_id = ` + r.bind(4) + `)
    OR (l.usage_attributed = ` + r.bind(5) + ` AND l.attributed_goal_id = ` + r.bind(6) + `)
  )
ORDER BY l.owner_user_id, l.goal_session_key, l.scope_round_id, l.source_round_id`
	if r.isPostgres {
		query += "\nFOR UPDATE OF l"
	}
	rows, err := tx.QueryContext(
		ctx,
		query,
		protocol.GoalUsageSourceKindNXSTask,
		false,
		false,
		strings.TrimSpace(goalID),
		true,
		strings.TrimSpace(goalID),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var pendingCount, unavailableCount int64
	for rows.Next() {
		var attributed, tokenObserved bool
		if err := rows.Scan(
			&attributed,
			&tokenObserved,
		); err != nil {
			return err
		}
		if !attributed {
			pendingCount++
			continue
		}
		if !tokenObserved {
			unavailableCount++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if pendingCount > 0 {
		return fmt.Errorf(
			"%w: goal %q has %d unclaimed Room parent usage rows",
			ErrGoalUsagePending,
			goalID,
			pendingCount,
		)
	}
	if unavailableCount > 0 {
		return fmt.Errorf(
			"%w: goal %q has %d Room terminal rows without authoritative parent usage",
			ErrGoalUsageUnavailable,
			goalID,
			unavailableCount,
		)
	}
	return nil
}

func (r *Repository) addUsageSourceGoalUsage(
	ctx context.Context,
	tx *sql.Tx,
	item *protocol.Goal,
	delta protocol.GoalUsage,
	observedAt time.Time,
) error {
	if item == nil {
		return fmt.Errorf("goal usage aggregate target is required")
	}
	delta = delta.NormalizeTotals()
	nextUsage := item.Usage.Add(delta)
	nextTime := item.TimeUsedSeconds + max(delta.RuntimeSeconds, int64(0))
	query := `UPDATE session_goals
SET token_used_input = ` + r.bind(1) + `,
    token_used_output = ` + r.bind(2) + `,
    token_used_cache_creation = ` + r.bind(3) + `,
    token_used_cache_read = ` + r.bind(4) + `,
    token_used_reasoning = ` + r.bind(5) + `,
    token_used_total = ` + r.bind(6) + `,
    token_used_actual_total = ` + r.bind(7) + `,
    token_used_actual_estimated = ` + r.bind(8) + `,
    time_used_seconds = ` + r.bind(9) + `,
    version = version + 1,
    updated_at = ` + r.bind(10) + `
WHERE goal_id = ` + r.bind(11) + `
  AND version = ` + r.bind(12) + `
  AND usage_finalized = ` + r.bind(13)
	result, err := tx.ExecContext(
		ctx,
		query,
		nextUsage.InputTokens,
		nextUsage.OutputTokens,
		nextUsage.CacheCreationInputTokens,
		nextUsage.CacheReadInputTokens,
		nextUsage.ReasoningTokens,
		nextUsage.BudgetTokens(),
		nextUsage.ActualTokens(),
		nextUsage.ActualTokensAreEstimated(),
		nextTime,
		observedAt,
		item.ID,
		item.Version,
		false,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("%w: goal %q", errGoalUsageSourceFinalized, item.ID)
	}
	if err != nil {
		return err
	}
	item.Usage = nextUsage
	item.TimeUsedSeconds = nextTime
	item.Version++
	item.UpdatedAt = observedAt
	return nil
}

func usageParentGoalEvent(
	snapshot protocol.GoalUsageParentSnapshot,
	item protocol.Goal,
	usage protocol.GoalUsage,
) protocol.GoalEvent {
	return protocol.GoalEvent{
		ID:         snapshot.EventID,
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    snapshot.SourceRoundID,
		Payload: map[string]any{
			"usage": usage.NormalizeTotals(),
			"usage_source": map[string]any{
				"source_kind":          "room_parent",
				"scope_round_id":       snapshot.ScopeRoundID,
				"source_round_id":      snapshot.SourceRoundID,
				"token_usage_observed": snapshot.TokenUsageObserved,
				"attribution":          "scope_bound_parent_terminal",
			},
		},
		CreatedAt: snapshot.ObservedAt,
	}
}

func parentUsageSnapshotUnavailable(snapshot protocol.GoalUsageParentSnapshot) bool {
	return !snapshot.TokenUsageObserved
}

func parentUsageLedgerRowUnavailable(row goalUsageParentLedgerRow) bool {
	return !row.tokenUsageObserved
}

func hasStoredGoalUsageTokens(usage protocol.GoalUsage) bool {
	return usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.ReasoningTokens > 0 ||
		usage.BudgetTokens() > 0 ||
		usage.ActualTokens() > 0
}

// normalizeStoredActualTotal 兼容已经把矛盾 provider total=0 落库的记录。
// SQL 没有单独的 presence 列，因此零 breakdown 仍保留为权威零；只在存在
// 正数分项时撤销这个 0 的权威性，并让协议层按 breakdown 回算。
func normalizeStoredActualTotal(usage *protocol.GoalUsage) {
	if usage == nil {
		return
	}
	hasPositiveBreakdown := usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.ReasoningTokens > 0
	usage.ActualTotalKnown = usage.ActualTotalTokens > 0 || !hasPositiveBreakdown
	if usage.ActualTotalTokens <= 0 && hasPositiveBreakdown {
		usage.ActualTokensEstimated = true
	}
}

func isStoredGoalUsageZero(usage protocol.GoalUsage) bool {
	return !hasStoredGoalUsageTokens(usage) && usage.RuntimeSeconds <= 0
}
