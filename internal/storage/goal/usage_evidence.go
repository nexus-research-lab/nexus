// INPUT: nxs child task 的 scope/source-round 生命周期与 provider token presence。
// OUTPUT: 可跨 handoff/重启恢复的 durable evidence、from-now tombstone 与 finalization barrier。
// POS: child 数值 checkpoint/pending 之外的生命周期真相源。
package goal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// upsertGoalUsageSourceEvidence 单调推进 child lifecycle evidence。
//
// terminal/token presence 一旦观察到就不会回退；discarded tombstone 也不会被
// replay 复活。数值高水位仍由 goal_usage_source_checkpoints 独立负责。
func (r *Repository) upsertGoalUsageSourceEvidence(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	query := fmt.Sprintf(`INSERT INTO goal_usage_source_evidence (
    owner_user_id,
    runtime_session_key,
    source_kind,
    source_id,
    goal_session_key,
    scope_round_id,
    source_round_id,
    terminal_observed,
    token_usage_observed,
    baseline_unavailable,
    discarded,
    observed_at
) VALUES (%s)
ON CONFLICT (
    owner_user_id,
    runtime_session_key,
    source_kind,
    source_id,
    goal_session_key,
    scope_round_id,
    source_round_id
) DO UPDATE SET
    terminal_observed = CASE
        WHEN goal_usage_source_evidence.terminal_observed = `+r.bind(13)+`
          OR excluded.terminal_observed = `+r.bind(14)+`
        THEN `+r.bind(15)+`
        ELSE `+r.bind(16)+`
    END,
    token_usage_observed = CASE
        WHEN goal_usage_source_evidence.baseline_unavailable = `+r.bind(17)+`
        THEN `+r.bind(18)+`
        WHEN goal_usage_source_evidence.token_usage_observed = `+r.bind(19)+`
          OR excluded.token_usage_observed = `+r.bind(20)+`
        THEN `+r.bind(21)+`
        ELSE `+r.bind(22)+`
    END,
    observed_at = CASE
        WHEN excluded.observed_at > goal_usage_source_evidence.observed_at
        THEN excluded.observed_at
        ELSE goal_usage_source_evidence.observed_at
    END`, r.bindList(12))
	_, err := tx.ExecContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.RoundID,
		snapshot.Terminal,
		snapshot.Terminal && snapshot.TokenUsageObserved,
		false,
		false,
		snapshot.ObservedAt,
		true,
		true,
		true,
		false,
		true,
		false,
		true,
		true,
		true,
		false,
	)
	return err
}

// discardTerminalGoalUsageSourceEvidence 把 from-now 绑定前已经终止的 child
// evidence 变成 durable tombstone。仍在运行的 child 保留并成为新 Goal 的 barrier。
func (r *Repository) discardTerminalGoalUsageSourceEvidence(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, error) {
	query := `UPDATE goal_usage_source_evidence
SET discarded = ` + r.bind(1) + `
WHERE owner_user_id = ` + r.bind(2) + `
  AND goal_session_key = ` + r.bind(3) + `
  AND source_kind = ` + r.bind(4) + `
  AND scope_round_id = ` + r.bind(5) + `
  AND terminal_observed = ` + r.bind(6) + `
  AND discarded = ` + r.bind(7)
	result, err := tx.ExecContext(
		ctx,
		query,
		true,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
		true,
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

// markGoalUsageSourceBaselinesUnavailable 把 external activation 时已经运行的
// child 持久标成无精确基线。旧 progress 即使为正也可能早于 activation，
// 所以后继累计 total 既不能归入新 Goal，也不能建立 authoritative fence。
func (r *Repository) markGoalUsageSourceBaselinesUnavailable(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, error) {
	query := `UPDATE goal_usage_source_evidence
SET baseline_unavailable = ` + r.bind(1) + `,
    token_usage_observed = ` + r.bind(2) + `
WHERE owner_user_id = ` + r.bind(3) + `
  AND goal_session_key = ` + r.bind(4) + `
  AND source_kind = ` + r.bind(5) + `
  AND scope_round_id = ` + r.bind(6) + `
  AND terminal_observed = ` + r.bind(7) + `
  AND discarded = ` + r.bind(8)
	result, err := tx.ExecContext(
		ctx,
		query,
		true,
		false,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
		false,
		false,
	)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return affected, err
}

func (r *Repository) markGoalUsageSourceSnapshotBaselineUnavailable(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	query := `UPDATE goal_usage_source_evidence
SET baseline_unavailable = ` + r.bind(1) + `,
    token_usage_observed = ` + r.bind(2) + `
WHERE owner_user_id = ` + r.bind(3) + `
  AND runtime_session_key = ` + r.bind(4) + `
  AND source_kind = ` + r.bind(5) + `
  AND source_id = ` + r.bind(6) + `
  AND goal_session_key = ` + r.bind(7) + `
  AND scope_round_id = ` + r.bind(8) + `
  AND source_round_id = ` + r.bind(9) + `
  AND discarded = ` + r.bind(10)
	_, err := tx.ExecContext(
		ctx,
		query,
		true,
		false,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.RoundID,
		false,
	)
	return err
}

func (r *Repository) discardGoalUsageSourceSnapshotEvidence(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	query := `UPDATE goal_usage_source_evidence
SET discarded = ` + r.bind(1) + `
WHERE owner_user_id = ` + r.bind(2) + `
  AND runtime_session_key = ` + r.bind(3) + `
  AND source_kind = ` + r.bind(4) + `
  AND source_id = ` + r.bind(5) + `
  AND goal_session_key = ` + r.bind(6) + `
  AND scope_round_id = ` + r.bind(7) + `
  AND source_round_id = ` + r.bind(8)
	_, err := tx.ExecContext(
		ctx,
		query,
		true,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.RoundID,
	)
	return err
}

func (r *Repository) goalUsageSourceBaselineUnavailable(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) (bool, error) {
	query := `SELECT baseline_unavailable
FROM goal_usage_source_evidence
WHERE owner_user_id = ` + r.bind(1) + `
  AND runtime_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND source_id = ` + r.bind(4) + `
  AND goal_session_key = ` + r.bind(5) + `
  AND scope_round_id = ` + r.bind(6) + `
  AND source_round_id = ` + r.bind(7)
	var unavailable bool
	err := tx.QueryRowContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
		snapshot.GoalSessionKey,
		snapshot.ScopeRoundID,
		snapshot.RoundID,
	).Scan(&unavailable)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return unavailable, err
}

// rejectGoalUsageSourceEvidenceFinalization 要求绑定 Goal 下每个 required child
// 都已终止且拿到正 provider total。运行中是可重试 pending；终态仍无 token 是
// terminal unavailable，调用方可停止重试但不能把 usage 标成 authoritative。
func (r *Repository) rejectGoalUsageSourceEvidenceFinalization(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
) error {
	query := `SELECT
    e.terminal_observed,
    e.token_usage_observed,
    e.baseline_unavailable
FROM goal_usage_source_evidence e
JOIN goal_usage_scope_bindings b
  ON b.owner_user_id = e.owner_user_id
 AND b.goal_session_key = e.goal_session_key
 AND b.source_kind = e.source_kind
 AND b.scope_round_id = e.scope_round_id
WHERE b.goal_id = ` + r.bind(1) + `
  AND e.discarded = ` + r.bind(2) + `
ORDER BY
    e.owner_user_id,
    e.runtime_session_key,
    e.source_kind,
    e.source_id,
    e.goal_session_key,
    e.scope_round_id,
    e.source_round_id`
	if r.isPostgres {
		query += "\nFOR UPDATE OF e"
	}
	rows, err := tx.QueryContext(ctx, query, strings.TrimSpace(goalID), false)
	if err != nil {
		return err
	}
	defer rows.Close()

	var nonTerminalCount, unavailableCount int64
	for rows.Next() {
		var terminalObserved, tokenUsageObserved, baselineUnavailable bool
		if err := rows.Scan(
			&terminalObserved,
			&tokenUsageObserved,
			&baselineUnavailable,
		); err != nil {
			return err
		}
		if !terminalObserved {
			nonTerminalCount++
			continue
		}
		if !tokenUsageObserved || baselineUnavailable {
			unavailableCount++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if nonTerminalCount > 0 {
		return fmt.Errorf(
			"%w: goal %q has %d non-terminal child usage evidence rows",
			ErrGoalUsagePending,
			goalID,
			nonTerminalCount,
		)
	}
	if unavailableCount > 0 {
		return fmt.Errorf(
			"%w: goal %q has %d terminal child rows without provider token usage",
			ErrGoalUsageUnavailable,
			goalID,
			unavailableCount,
		)
	}
	return nil
}
