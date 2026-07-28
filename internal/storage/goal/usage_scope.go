// INPUT: Goal usage scope 身份、规范化 pending delta 与可选 Goal 绑定。
// OUTPUT: scope 级串行化、跨 runtime pending 认领、不可改绑约束及 finalization barrier。
// POS: usage_source.go 与 usage_finalization.go 共享的 durable scope SQL 原语。
package goal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalUsageScopeKey struct {
	ownerUserID    string
	goalSessionKey string
	sourceKind     string
	scopeRoundID   string
}

type goalUsageScopeRow struct {
	state        string
	goalID       sql.NullString
	boundAt      sql.NullTime
	closedAt     sql.NullTime
	usageEventID sql.NullString
}

type goalUsageScopeResolution struct {
	goalID       string
	closed       bool
	boundAt      time.Time
	usageEventID string
}

func goalUsageScopeKeyFromSnapshot(snapshot protocol.GoalUsageSourceSnapshot) goalUsageScopeKey {
	return goalUsageScopeKey{
		ownerUserID:    snapshot.OwnerUserID,
		goalSessionKey: snapshot.GoalSessionKey,
		sourceKind:     snapshot.SourceKind,
		scopeRoundID:   snapshot.ScopeRoundID,
	}
}

func goalUsageScopeKeyFromBinding(binding protocol.GoalUsageScopeBinding) goalUsageScopeKey {
	return goalUsageScopeKey{
		ownerUserID:    binding.OwnerUserID,
		goalSessionKey: binding.GoalSessionKey,
		sourceKind:     binding.SourceKind,
		scopeRoundID:   binding.ScopeRoundID,
	}
}

func (r *Repository) peekGoalUsageScope(
	ctx context.Context,
	key goalUsageScopeKey,
) (goalUsageScopeResolution, error) {
	query := `SELECT state, goal_id, bound_at, usage_event_id
FROM goal_usage_scope_bindings
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4)
	var state string
	var goalID sql.NullString
	var boundAt sql.NullTime
	var usageEventID sql.NullString
	err := r.db.QueryRowContext(
		ctx,
		query,
		key.ownerUserID,
		key.goalSessionKey,
		key.sourceKind,
		key.scopeRoundID,
	).Scan(&state, &goalID, &boundAt, &usageEventID)
	if err == sql.ErrNoRows {
		return goalUsageScopeResolution{}, nil
	}
	if err != nil {
		return goalUsageScopeResolution{}, err
	}
	return goalUsageScopeResolution{
		goalID:       strings.TrimSpace(goalID.String),
		closed:       state == "closed" || (state == "bound" && !goalID.Valid),
		boundAt:      boundAt.Time.UTC(),
		usageEventID: strings.TrimSpace(usageEventID.String),
	}, nil
}

func normalizeGoalUsageScopeBinding(binding protocol.GoalUsageScopeBinding) protocol.GoalUsageScopeBinding {
	binding.OwnerUserID = strings.TrimSpace(binding.OwnerUserID)
	binding.GoalSessionKey = strings.TrimSpace(binding.GoalSessionKey)
	binding.SourceKind = strings.TrimSpace(binding.SourceKind)
	binding.ScopeRoundID = strings.TrimSpace(binding.ScopeRoundID)
	binding.GoalID = strings.TrimSpace(binding.GoalID)
	binding.UsageEventID = strings.TrimSpace(binding.UsageEventID)
	binding.BoundAt = binding.BoundAt.UTC()
	return binding
}

func validateGoalUsageScopeBinding(binding protocol.GoalUsageScopeBinding) error {
	if binding.OwnerUserID == "" ||
		binding.GoalSessionKey == "" ||
		binding.ScopeRoundID == "" ||
		binding.GoalID == "" {
		return fmt.Errorf("goal usage scope binding identity is incomplete")
	}
	if binding.SourceKind != protocol.GoalUsageSourceKindNXSTask {
		return fmt.Errorf("unsupported goal usage source kind %q", binding.SourceKind)
	}
	return nil
}

// lockGoalUsageScope serializes a snapshot against a concurrent Goal create, then
// resolves the optional binding. closed scopes keep advancing checkpoints but never
// create new pending rows and can never be rebound.
func (r *Repository) lockGoalUsageScope(
	ctx context.Context,
	tx *sql.Tx,
	key goalUsageScopeKey,
) (goalUsageScopeResolution, error) {
	row, err := r.ensureAndLockGoalUsageScope(ctx, tx, key)
	if err != nil {
		return goalUsageScopeResolution{}, err
	}
	if row.state == "open" {
		return goalUsageScopeResolution{}, nil
	}
	return goalUsageScopeResolution{
		goalID:       strings.TrimSpace(row.goalID.String),
		closed:       row.state == "closed" || !row.goalID.Valid,
		boundAt:      row.boundAt.Time.UTC(),
		usageEventID: strings.TrimSpace(row.usageEventID.String),
	}, nil
}

// excludesFromNowSnapshot 区分 external Reset 与 model round-start claim：
// external binding 没有 usage event，且观察时间不晚于绑定边界的 snapshot
// 属于旧工作。它即使在绑定事务之后才提交，也不能归入新 Goal。
func (resolution goalUsageScopeResolution) excludesFromNowSnapshot(observedAt time.Time) bool {
	return resolution.goalID != "" &&
		resolution.usageEventID == "" &&
		!resolution.boundAt.IsZero() &&
		!observedAt.After(resolution.boundAt)
}

func (r *Repository) lockExistingGoalUsageScope(
	ctx context.Context,
	tx *sql.Tx,
	key goalUsageScopeKey,
	expectedGoalID string,
) (bool, error) {
	query := `SELECT state, goal_id
FROM goal_usage_scope_bindings
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4)
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	var state string
	var goalID sql.NullString
	err := tx.QueryRowContext(
		ctx,
		query,
		key.ownerUserID,
		key.goalSessionKey,
		key.sourceKind,
		key.scopeRoundID,
	).Scan(&state, &goalID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == "bound" && strings.TrimSpace(goalID.String) == strings.TrimSpace(expectedGoalID), nil
}

func (r *Repository) ensureAndLockGoalUsageScope(
	ctx context.Context,
	tx *sql.Tx,
	key goalUsageScopeKey,
) (goalUsageScopeRow, error) {
	prefix := "INSERT OR IGNORE INTO"
	suffix := ""
	if r.isPostgres {
		prefix = "INSERT INTO"
		suffix = "\nON CONFLICT (owner_user_id, goal_session_key, source_kind, scope_round_id) DO NOTHING"
	}
	insert := fmt.Sprintf(`%s goal_usage_scope_bindings (
    owner_user_id,
    goal_session_key,
    source_kind,
    scope_round_id,
    state
) VALUES (%s)%s`, prefix, r.bindList(5), suffix)
	if _, err := tx.ExecContext(
		ctx,
		insert,
		key.ownerUserID,
		key.goalSessionKey,
		key.sourceKind,
		key.scopeRoundID,
		"open",
	); err != nil {
		return goalUsageScopeRow{}, err
	}

	query := `SELECT state, goal_id, bound_at, closed_at, usage_event_id
FROM goal_usage_scope_bindings
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4)
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	var row goalUsageScopeRow
	err := tx.QueryRowContext(
		ctx,
		query,
		key.ownerUserID,
		key.goalSessionKey,
		key.sourceKind,
		key.scopeRoundID,
	).Scan(&row.state, &row.goalID, &row.boundAt, &row.closedAt, &row.usageEventID)
	if err != nil {
		return goalUsageScopeRow{}, err
	}
	return row, nil
}

func (r *Repository) establishGoalUsageScopeBinding(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) error {
	_, err := r.establishGoalUsageScopeBindingWithStatus(ctx, tx, binding)
	return err
}

// establishGoalUsageScopeBindingWithStatus 除了不可改绑约束，还区分本事务
// 是否首次把 open scope 绑定。from-now backlog 只能在首次绑定时排除；
// 网络不确定后的幂等重试不能再次清理绑定后产生的 usage/evidence。
func (r *Repository) establishGoalUsageScopeBindingWithStatus(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (bool, error) {
	binding = normalizeGoalUsageScopeBinding(binding)
	if err := validateGoalUsageScopeBinding(binding); err != nil {
		return false, err
	}
	key := goalUsageScopeKeyFromBinding(binding)
	row, err := r.ensureAndLockGoalUsageScope(ctx, tx, key)
	if err != nil {
		return false, err
	}
	if row.state != "open" {
		if row.state == "closed" || !row.goalID.Valid || strings.TrimSpace(row.goalID.String) != binding.GoalID {
			return false, fmt.Errorf(
				"%w: owner=%q session=%q kind=%q scope=%q state=%q goal=%q cannot bind %q",
				ErrGoalUsageScopeConflict,
				binding.OwnerUserID,
				binding.GoalSessionKey,
				binding.SourceKind,
				binding.ScopeRoundID,
				row.state,
				row.goalID.String,
				binding.GoalID,
			)
		}
		return false, nil
	}

	query := `UPDATE goal_usage_scope_bindings
SET state = 'bound',
    goal_id = ` + r.bind(1) + `,
    bound_at = ` + r.bind(2) + `,
    closed_at = NULL,
    usage_event_id = ` + r.bind(3) + `
WHERE owner_user_id = ` + r.bind(4) + `
  AND goal_session_key = ` + r.bind(5) + `
  AND source_kind = ` + r.bind(6) + `
  AND scope_round_id = ` + r.bind(7) + `
  AND state = 'open'`
	result, err := tx.ExecContext(
		ctx,
		query,
		binding.GoalID,
		binding.BoundAt,
		nullString(binding.UsageEventID),
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != 1 {
		return false, fmt.Errorf("goal usage scope binding update affected %d rows, want 1", affected)
	}
	return err == nil, err
}

func (r *Repository) validateGoalUsageScopeOwner(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
	ownerUserID string,
) error {
	query := `SELECT DISTINCT owner_user_id
FROM goal_usage_scope_bindings
WHERE goal_id = ` + r.bind(1) + `
  AND state = 'bound'
ORDER BY owner_user_id`
	rows, err := tx.QueryContext(ctx, query, strings.TrimSpace(goalID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var storedOwner string
		if err := rows.Scan(&storedOwner); err != nil {
			return err
		}
		if strings.TrimSpace(storedOwner) != strings.TrimSpace(ownerUserID) {
			return fmt.Errorf(
				"%w: goal %q is owned by usage scope owner %q, not %q",
				ErrGoalUsageScopeConflict,
				goalID,
				storedOwner,
				ownerUserID,
			)
		}
	}
	return rows.Err()
}

func (r *Repository) addGoalUsageSourcePending(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
	delta int64,
) error {
	query := fmt.Sprintf(`INSERT INTO goal_usage_source_pending (
    owner_user_id,
    runtime_session_key,
    source_kind,
    source_id,
    goal_session_key,
    scope_round_id,
    source_round_id,
    pending_actual_tokens,
    last_observed_at
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
    pending_actual_tokens = goal_usage_source_pending.pending_actual_tokens + excluded.pending_actual_tokens,
    last_observed_at = excluded.last_observed_at`, r.bindList(9))
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
		delta,
		snapshot.ObservedAt,
	)
	return err
}

func (r *Repository) discardGoalUsageSourcePending(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, error) {
	query := `DELETE FROM goal_usage_source_pending
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4)
	result, err := tx.ExecContext(
		ctx,
		query,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
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

func (r *Repository) lockGoalUsageScopePending(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
) (int64, int64, error) {
	query := `SELECT pending_actual_tokens
FROM goal_usage_source_pending
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4) + `
ORDER BY runtime_session_key, source_id, source_round_id`
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	rows, err := tx.QueryContext(
		ctx,
		query,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var pendingCount, pendingTokens int64
	for rows.Next() {
		var current int64
		if err := rows.Scan(&current); err != nil {
			return 0, 0, err
		}
		pendingCount++
		pendingTokens += current
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	return pendingCount, pendingTokens, nil
}

func (r *Repository) deleteGoalUsageScopePending(
	ctx context.Context,
	tx *sql.Tx,
	binding protocol.GoalUsageScopeBinding,
	expectedRows int64,
) error {
	markQuery := `UPDATE goal_usage_source_checkpoints
SET last_attributed_goal_id = ` + r.bind(1) + `,
    last_attributed_round_id = ` + r.bind(2) + `
WHERE EXISTS (
    SELECT 1
    FROM goal_usage_source_pending
    WHERE goal_usage_source_pending.owner_user_id = goal_usage_source_checkpoints.owner_user_id
      AND goal_usage_source_pending.runtime_session_key = goal_usage_source_checkpoints.runtime_session_key
      AND goal_usage_source_pending.source_kind = goal_usage_source_checkpoints.source_kind
      AND goal_usage_source_pending.source_id = goal_usage_source_checkpoints.source_id
      AND goal_usage_source_pending.owner_user_id = ` + r.bind(3) + `
      AND goal_usage_source_pending.goal_session_key = ` + r.bind(4) + `
      AND goal_usage_source_pending.source_kind = ` + r.bind(5) + `
      AND goal_usage_source_pending.scope_round_id = ` + r.bind(6) + `
)`
	if _, err := tx.ExecContext(
		ctx,
		markQuery,
		binding.GoalID,
		binding.ScopeRoundID,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
	); err != nil {
		return err
	}

	query := `DELETE FROM goal_usage_source_pending
WHERE owner_user_id = ` + r.bind(1) + `
  AND goal_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND scope_round_id = ` + r.bind(4)
	result, err := tx.ExecContext(
		ctx,
		query,
		binding.OwnerUserID,
		binding.GoalSessionKey,
		binding.SourceKind,
		binding.ScopeRoundID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != expectedRows {
		return fmt.Errorf("goal usage scope claim deleted %d pending rows, expected %d", affected, expectedRows)
	}
	return err
}

func (r *Repository) rejectGoalUsageFinalizationWithPending(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
) error {
	bindingsQuery := `SELECT owner_user_id, goal_session_key, source_kind, scope_round_id
FROM goal_usage_scope_bindings
WHERE goal_id = ` + r.bind(1) + `
ORDER BY owner_user_id, goal_session_key, source_kind, scope_round_id`
	if r.isPostgres {
		bindingsQuery += "\nFOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, bindingsQuery, strings.TrimSpace(goalID))
	if err != nil {
		return err
	}
	for rows.Next() {
		var key goalUsageScopeKey
		if err := rows.Scan(&key.ownerUserID, &key.goalSessionKey, &key.sourceKind, &key.scopeRoundID); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	pendingQuery := `SELECT p.pending_actual_tokens
FROM goal_usage_source_pending p
JOIN goal_usage_scope_bindings b
  ON b.owner_user_id = p.owner_user_id
 AND b.goal_session_key = p.goal_session_key
 AND b.source_kind = p.source_kind
 AND b.scope_round_id = p.scope_round_id
WHERE b.goal_id = ` + r.bind(1) + `
ORDER BY p.owner_user_id, p.runtime_session_key, p.source_kind, p.source_id, p.scope_round_id, p.source_round_id`
	if r.isPostgres {
		pendingQuery += "\nFOR UPDATE OF p"
	}
	pendingRows, err := tx.QueryContext(ctx, pendingQuery, strings.TrimSpace(goalID))
	if err != nil {
		return err
	}
	defer pendingRows.Close()
	var pendingTokens int64
	for pendingRows.Next() {
		var current int64
		if err := pendingRows.Scan(&current); err != nil {
			return err
		}
		pendingTokens += current
	}
	if err := pendingRows.Err(); err != nil {
		return err
	}
	if pendingTokens > 0 {
		return fmt.Errorf("%w: goal %q has %d unclaimed child tokens", ErrGoalUsagePending, goalID, pendingTokens)
	}
	if err := r.rejectGoalUsageSourceEvidenceFinalization(ctx, tx, goalID); err != nil {
		return err
	}
	return r.rejectGoalUsageParentFinalization(ctx, tx, goalID)
}
