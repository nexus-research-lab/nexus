// INPUT: runtime source 的单调累计 actual token、durable scope、固定 Goal 绑定与审计事件身份。
// OUTPUT: checkpoint 高水位、scope pending/绑定、finalized fence、Goal actual usage/version 和审计事件的单事务结果。
// POS: nxs child usage 跨进程去重并原子归属 Goal 的 SQL 边界。
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

var (
	errGoalUsageSourceFinalized = errors.New("goal usage source attribution rejected because goal usage is finalized")
	errGoalUsageScopeRetry      = errors.New("goal usage scope changed while snapshot was starting")

	// ErrGoalUsageScopeConflict 表示 durable scope 已绑定到另一个 Goal。
	ErrGoalUsageScopeConflict = errors.New("goal usage scope is already bound to another goal")
	// ErrGoalUsagePending 表示 Goal 的 durable scope 下仍有未归属的 child usage。
	ErrGoalUsagePending = errors.New("goal usage cannot be finalized while child usage is pending")
	// ErrGoalUsageUnavailable 表示至少一个 parent/child terminal 未提供 provider usage。
	ErrGoalUsageUnavailable = errors.New("goal usage cannot be finalized because terminal token usage is unavailable")
)

type goalUsageSourceCheckpoint struct {
	cumulativeActualTokens int64
}

// ApplyUsageSourceSnapshot 原子推进 source checkpoint。无显式 Goal 时先解析
// durable scope binding；未绑定的 delta 写入规范化 pending，已绑定的 delta 直接归属。
func (r *Repository) ApplyUsageSourceSnapshot(
	ctx context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	snapshot = normalizeGoalUsageSourceSnapshot(snapshot)
	if err := validateGoalUsageSourceSnapshot(snapshot); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		result, err := r.applyUsageSourceSnapshotOnce(ctx, snapshot)
		if !errors.Is(err, errGoalUsageScopeRetry) {
			return result, err
		}
	}
	return protocol.GoalUsageSourceResult{}, fmt.Errorf("%w after retries", errGoalUsageScopeRetry)
}

func (r *Repository) applyUsageSourceSnapshotOnce(
	ctx context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	explicitGoalBinding := snapshot.GoalID != ""
	resolution, err := r.peekGoalUsageScope(ctx, goalUsageScopeKeyFromSnapshot(snapshot))
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var item *protocol.Goal
	var binding *protocol.GoalUsageScopeBinding
	newlyBound := false
	if resolution.closed {
		lockedResolution, resolveErr := r.lockGoalUsageScope(
			ctx,
			tx,
			goalUsageScopeKeyFromSnapshot(snapshot),
		)
		if resolveErr != nil {
			return protocol.GoalUsageSourceResult{}, resolveErr
		}
		if !lockedResolution.closed {
			return protocol.GoalUsageSourceResult{}, errGoalUsageScopeRetry
		}
		resolution = lockedResolution
		snapshot.GoalID = ""
	} else if snapshot.GoalID != "" {
		item, err = r.lockUsageSourceGoal(ctx, tx, snapshot.GoalID)
		if errors.Is(err, sql.ErrNoRows) && resolution.goalID == snapshot.GoalID {
			return protocol.GoalUsageSourceResult{}, errGoalUsageScopeRetry
		}
		if err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if err := validateUsageSourceGoal(*item, snapshot.GoalSessionKey); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if snapshot.EventID == "" {
			return protocol.GoalUsageSourceResult{}, fmt.Errorf("goal usage source event id is required")
		}
		if err := r.validateGoalUsageScopeOwner(ctx, tx, item.ID, snapshot.OwnerUserID); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		currentBinding := protocol.GoalUsageScopeBinding{
			OwnerUserID:    snapshot.OwnerUserID,
			GoalSessionKey: snapshot.GoalSessionKey,
			SourceKind:     snapshot.SourceKind,
			ScopeRoundID:   snapshot.ScopeRoundID,
			GoalID:         item.ID,
			BoundAt:        snapshot.ObservedAt,
			UsageEventID:   snapshot.EventID,
		}
		newlyBound, err = r.establishGoalUsageScopeBindingWithStatus(ctx, tx, currentBinding)
		if err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		binding = &currentBinding
	} else if resolution.goalID != "" {
		item, err = r.lockUsageSourceGoal(ctx, tx, resolution.goalID)
		if errors.Is(err, sql.ErrNoRows) {
			return protocol.GoalUsageSourceResult{}, errGoalUsageScopeRetry
		}
		if err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if err := validateUsageSourceGoal(*item, snapshot.GoalSessionKey); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if snapshot.EventID == "" {
			return protocol.GoalUsageSourceResult{}, fmt.Errorf("goal usage source event id is required")
		}
		locked, resolveErr := r.lockExistingGoalUsageScope(
			ctx,
			tx,
			goalUsageScopeKeyFromSnapshot(snapshot),
			resolution.goalID,
		)
		if resolveErr != nil {
			return protocol.GoalUsageSourceResult{}, resolveErr
		}
		if !locked {
			return protocol.GoalUsageSourceResult{}, errGoalUsageScopeRetry
		}
		snapshot.GoalID = item.ID
		currentBinding := protocol.GoalUsageScopeBinding{
			OwnerUserID:    snapshot.OwnerUserID,
			GoalSessionKey: snapshot.GoalSessionKey,
			SourceKind:     snapshot.SourceKind,
			ScopeRoundID:   snapshot.ScopeRoundID,
			GoalID:         item.ID,
			BoundAt:        snapshot.ObservedAt,
			UsageEventID:   snapshot.EventID,
		}
		binding = &currentBinding
	} else {
		lockedResolution, resolveErr := r.lockGoalUsageScope(
			ctx,
			tx,
			goalUsageScopeKeyFromSnapshot(snapshot),
		)
		if resolveErr != nil {
			return protocol.GoalUsageSourceResult{}, resolveErr
		}
		if lockedResolution.goalID != "" {
			return protocol.GoalUsageSourceResult{}, errGoalUsageScopeRetry
		}
		resolution = lockedResolution
	}

	if item != nil && binding != nil && explicitGoalBinding && newlyBound {
		// 普通 snapshot 携带/解析到 Goal 时，只从当前 checkpoint delta
		// 开始归属。round-start backlog 仅由 atomic Create/Claim 显式认领；
		// external activation 前或异常遗留的 pending 必须清理但不能计入。
		// 只在首次绑定执行，幂等重试不能误丢绑定后新证据。
		if _, err := r.discardGoalUsageSourcePending(ctx, tx, *binding); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if _, err := r.discardTerminalGoalUsageSourceEvidence(ctx, tx, *binding); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if _, err := r.discardGoalUsageParentPending(ctx, tx, *binding); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	preBindingFromNow := resolution.excludesFromNowSnapshot(snapshot.ObservedAt)
	baselineUnavailable := false
	if !resolution.closed && snapshot.EvidenceRequired {
		if err := r.upsertGoalUsageSourceEvidence(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		if preBindingFromNow {
			if snapshot.Terminal {
				if err := r.discardGoalUsageSourceSnapshotEvidence(ctx, tx, snapshot); err != nil {
					return protocol.GoalUsageSourceResult{}, err
				}
			} else if err := r.markGoalUsageSourceSnapshotBaselineUnavailable(
				ctx,
				tx,
				snapshot,
			); err != nil {
				return protocol.GoalUsageSourceResult{}, err
			}
		}
		baselineUnavailable, err = r.goalUsageSourceBaselineUnavailable(ctx, tx, snapshot)
		if err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	if err := r.ensureGoalUsageSourceCheckpoint(ctx, tx, snapshot); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	previous, err := r.lockGoalUsageSourceCheckpoint(ctx, tx, snapshot)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	delta := int64(0)
	if snapshot.CumulativeActualTokens > previous.cumulativeActualTokens {
		delta = snapshot.CumulativeActualTokens - previous.cumulativeActualTokens
		if err := r.advanceGoalUsageSourceCheckpoint(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	tokenUsageUnavailable := snapshot.Terminal &&
		(!snapshot.TokenUsageObserved || baselineUnavailable)
	if delta == 0 {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		return protocol.GoalUsageSourceResult{
			TokenUsageUnavailable: tokenUsageUnavailable,
		}, nil
	}

	result := protocol.GoalUsageSourceResult{
		ObservedDelta:         delta,
		TokenUsageUnavailable: tokenUsageUnavailable,
	}
	if preBindingFromNow || baselineUnavailable {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		return result, nil
	}
	if resolution.closed {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		return result, nil
	}
	if item == nil {
		if delta > 0 {
			if err := r.addGoalUsageSourcePending(ctx, tx, snapshot, delta); err != nil {
				return protocol.GoalUsageSourceResult{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		return result, nil
	}

	attributedDelta := delta
	if err := r.addUsageSourceGoalActualTokens(ctx, tx, item.ID, attributedDelta, snapshot.ObservedAt); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if delta > 0 {
		if err := r.markGoalUsageSourceAttributed(ctx, tx, snapshot); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	event := usageSourceGoalEvent(snapshot, *item, attributedDelta)
	if err := r.insertGoalEvent(ctx, tx, event); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	updated, err := r.getUsageSourceGoal(ctx, tx, item.ID)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	result.AttributedDelta = attributedDelta
	result.AttributedUsage = (protocol.GoalUsage{
		ActualTotalTokens: attributedDelta,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}).NormalizeTotals()
	result.Goal = updated
	result.Event = &event
	return result, nil
}

// ClaimUsageSourceRound 建立 scope 到 Goal 的 durable 绑定，并跨所有 runtime
// session 认领该 scope 的 pending。重复认领同一绑定是 no-op，scope 不可改绑。
func (r *Repository) ClaimUsageSourceRound(
	ctx context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	claim = normalizeGoalUsageSourceRoundClaim(claim)
	if err := validateGoalUsageSourceRoundClaim(claim); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	binding := protocol.GoalUsageScopeBinding{
		OwnerUserID:    claim.OwnerUserID,
		GoalSessionKey: claim.GoalSessionKey,
		SourceKind:     claim.SourceKind,
		ScopeRoundID:   claim.ScopeRoundID,
		GoalID:         claim.GoalID,
		BoundAt:        claim.ClaimedAt,
		UsageEventID:   claim.EventID,
	}
	item, err := r.lockUsageSourceGoal(ctx, tx, claim.GoalID)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if err := validateUsageSourceGoal(*item, claim.GoalSessionKey); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if err := r.validateGoalUsageScopeOwner(ctx, tx, item.ID, claim.OwnerUserID); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if err := r.establishGoalUsageScopeBinding(ctx, tx, binding); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}

	pendingCount, pendingTokens, err := r.lockGoalUsageScopePending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	parentCount, parentUnavailable, parentUsage, err := r.lockGoalUsageParentPending(ctx, tx, binding)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	childUsage := (protocol.GoalUsage{
		ActualTotalTokens: pendingTokens,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}).NormalizeTotals()
	attributedUsage := parentUsage.Add(childUsage)
	if pendingCount == 0 && parentCount == 0 {
		if err := tx.Commit(); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		return protocol.GoalUsageSourceResult{}, nil
	}
	if !isStoredGoalUsageZero(attributedUsage) && claim.EventID == "" {
		return protocol.GoalUsageSourceResult{}, fmt.Errorf("goal usage source claim event id is required")
	}
	if !isStoredGoalUsageZero(attributedUsage) {
		if err := r.addUsageSourceGoalUsage(ctx, tx, item, attributedUsage, claim.ClaimedAt); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	if pendingCount > 0 {
		if err := r.deleteGoalUsageScopePending(ctx, tx, binding, pendingCount); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	if parentCount > 0 {
		if err := r.markGoalUsageParentScopeAttributed(ctx, tx, binding, parentCount); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
	}
	var event *protocol.GoalEvent
	if !isStoredGoalUsageZero(attributedUsage) {
		current := usageSourceScopeClaimEvent(
			binding,
			*item,
			claim.RoundID,
			attributedUsage,
			pendingCount,
			parentCount,
			parentUnavailable,
			"scope_claim_backfill",
		)
		if err := r.insertGoalEvent(ctx, tx, current); err != nil {
			return protocol.GoalUsageSourceResult{}, err
		}
		event = &current
	}
	updated, err := r.getUsageSourceGoal(ctx, tx, item.ID)
	if err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.GoalUsageSourceResult{}, err
	}
	return protocol.GoalUsageSourceResult{
		AttributedDelta:       pendingTokens,
		AttributedUsage:       attributedUsage,
		TokenUsageUnavailable: parentUnavailable > 0,
		Goal:                  updated,
		Event:                 event,
	}, nil
}

func normalizeGoalUsageSourceSnapshot(snapshot protocol.GoalUsageSourceSnapshot) protocol.GoalUsageSourceSnapshot {
	snapshot.OwnerUserID = strings.TrimSpace(snapshot.OwnerUserID)
	snapshot.RuntimeSessionKey = strings.TrimSpace(snapshot.RuntimeSessionKey)
	snapshot.SourceKind = strings.TrimSpace(snapshot.SourceKind)
	snapshot.SourceID = strings.TrimSpace(snapshot.SourceID)
	snapshot.GoalID = strings.TrimSpace(snapshot.GoalID)
	snapshot.GoalSessionKey = strings.TrimSpace(snapshot.GoalSessionKey)
	snapshot.RoundID = strings.TrimSpace(snapshot.RoundID)
	snapshot.ScopeRoundID = strings.TrimSpace(snapshot.ScopeRoundID)
	if snapshot.ScopeRoundID == "" {
		snapshot.ScopeRoundID = snapshot.RoundID
	}
	if snapshot.RoundID == "" {
		snapshot.RoundID = snapshot.ScopeRoundID
	}
	snapshot.EventID = strings.TrimSpace(snapshot.EventID)
	snapshot.ObservedAt = snapshot.ObservedAt.UTC()
	snapshot.EvidenceRequired = snapshot.EvidenceRequired ||
		snapshot.Terminal ||
		snapshot.TokenUsageObserved
	return snapshot
}

func validateGoalUsageSourceSnapshot(snapshot protocol.GoalUsageSourceSnapshot) error {
	if snapshot.OwnerUserID == "" ||
		snapshot.RuntimeSessionKey == "" ||
		snapshot.SourceID == "" ||
		snapshot.GoalSessionKey == "" ||
		snapshot.ScopeRoundID == "" ||
		snapshot.RoundID == "" {
		return fmt.Errorf("goal usage source snapshot identity is incomplete")
	}
	if snapshot.SourceKind != protocol.GoalUsageSourceKindNXSTask {
		return fmt.Errorf("unsupported goal usage source kind %q", snapshot.SourceKind)
	}
	if snapshot.CumulativeActualTokens < 0 {
		return fmt.Errorf("goal usage source cumulative tokens must be non-negative")
	}
	if snapshot.TokenUsageObserved &&
		(!snapshot.Terminal || snapshot.CumulativeActualTokens <= 0) {
		return fmt.Errorf("goal usage source token evidence requires a positive terminal snapshot")
	}
	return nil
}

func normalizeGoalUsageSourceRoundClaim(claim protocol.GoalUsageSourceRoundClaim) protocol.GoalUsageSourceRoundClaim {
	claim.OwnerUserID = strings.TrimSpace(claim.OwnerUserID)
	claim.RuntimeSessionKey = strings.TrimSpace(claim.RuntimeSessionKey)
	claim.SourceKind = strings.TrimSpace(claim.SourceKind)
	claim.RoundID = strings.TrimSpace(claim.RoundID)
	claim.ScopeRoundID = strings.TrimSpace(claim.ScopeRoundID)
	if claim.ScopeRoundID == "" {
		claim.ScopeRoundID = claim.RoundID
	}
	if claim.RoundID == "" {
		claim.RoundID = claim.ScopeRoundID
	}
	claim.GoalID = strings.TrimSpace(claim.GoalID)
	claim.GoalSessionKey = strings.TrimSpace(claim.GoalSessionKey)
	claim.EventID = strings.TrimSpace(claim.EventID)
	claim.ClaimedAt = claim.ClaimedAt.UTC()
	return claim
}

func validateGoalUsageSourceRoundClaim(claim protocol.GoalUsageSourceRoundClaim) error {
	if claim.OwnerUserID == "" ||
		claim.GoalSessionKey == "" ||
		claim.ScopeRoundID == "" ||
		claim.GoalID == "" {
		return fmt.Errorf("goal usage source claim identity is incomplete")
	}
	if claim.SourceKind != protocol.GoalUsageSourceKindNXSTask {
		return fmt.Errorf("unsupported goal usage source kind %q", claim.SourceKind)
	}
	return nil
}

func (r *Repository) ensureGoalUsageSourceCheckpoint(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	prefix := "INSERT OR IGNORE INTO"
	suffix := ""
	if r.isPostgres {
		prefix = "INSERT INTO"
		suffix = "\nON CONFLICT (owner_user_id, runtime_session_key, source_kind, source_id) DO NOTHING"
	}
	query := fmt.Sprintf(`%s goal_usage_source_checkpoints (
    owner_user_id,
    runtime_session_key,
    source_kind,
    source_id,
    cumulative_actual_tokens,
    last_observed_at
) VALUES (%s)%s`, prefix, r.bindList(6), suffix)
	_, err := tx.ExecContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
		int64(0),
		snapshot.ObservedAt,
	)
	return err
}

func (r *Repository) lockGoalUsageSourceCheckpoint(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) (goalUsageSourceCheckpoint, error) {
	query := `SELECT cumulative_actual_tokens
FROM goal_usage_source_checkpoints
WHERE owner_user_id = ` + r.bind(1) + `
  AND runtime_session_key = ` + r.bind(2) + `
  AND source_kind = ` + r.bind(3) + `
  AND source_id = ` + r.bind(4)
	if r.isPostgres {
		query += "\nFOR UPDATE"
	}
	var checkpoint goalUsageSourceCheckpoint
	err := tx.QueryRowContext(
		ctx,
		query,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
	).Scan(&checkpoint.cumulativeActualTokens)
	return checkpoint, err
}

func (r *Repository) advanceGoalUsageSourceCheckpoint(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	query := `UPDATE goal_usage_source_checkpoints
SET cumulative_actual_tokens = ` + r.bind(1) + `,
    last_observed_at = ` + r.bind(2) + `
WHERE owner_user_id = ` + r.bind(3) + `
  AND runtime_session_key = ` + r.bind(4) + `
  AND source_kind = ` + r.bind(5) + `
  AND source_id = ` + r.bind(6)
	_, err := tx.ExecContext(
		ctx,
		query,
		snapshot.CumulativeActualTokens,
		snapshot.ObservedAt,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
	)
	return err
}

func (r *Repository) lockUsageSourceGoal(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
) (*protocol.Goal, error) {
	where := "goal_id = " + r.bind(1)
	if r.isPostgres {
		where += " FOR UPDATE"
	}
	item, err := scanGoal(tx.QueryRowContext(ctx, goalSelectQuery(where), goalID))
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func validateUsageSourceGoal(item protocol.Goal, goalSessionKey string) error {
	if strings.TrimSpace(item.SessionKey) != strings.TrimSpace(goalSessionKey) {
		return fmt.Errorf(
			"goal usage source session mismatch: goal %q belongs to %q, usage scope uses %q",
			item.ID,
			item.SessionKey,
			goalSessionKey,
		)
	}
	if item.UsageFinalized {
		return fmt.Errorf("%w: goal %q", errGoalUsageSourceFinalized, item.ID)
	}
	return nil
}

func (r *Repository) addUsageSourceGoalActualTokens(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
	delta int64,
	observedAt time.Time,
) error {
	query := `UPDATE session_goals
SET token_used_actual_total = token_used_actual_total + ` + r.bind(1) + `,
    version = version + 1,
    updated_at = ` + r.bind(2) + `
WHERE goal_id = ` + r.bind(3) + `
  AND usage_finalized = ` + r.bind(4)
	result, err := tx.ExecContext(ctx, query, delta, observedAt, goalID, false)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("%w: goal %q", errGoalUsageSourceFinalized, goalID)
	}
	return err
}

func (r *Repository) markGoalUsageSourceAttributed(
	ctx context.Context,
	tx *sql.Tx,
	snapshot protocol.GoalUsageSourceSnapshot,
) error {
	query := `UPDATE goal_usage_source_checkpoints
SET last_attributed_goal_id = ` + r.bind(1) + `,
    last_attributed_round_id = ` + r.bind(2) + `
WHERE owner_user_id = ` + r.bind(3) + `
  AND runtime_session_key = ` + r.bind(4) + `
  AND source_kind = ` + r.bind(5) + `
  AND source_id = ` + r.bind(6)
	_, err := tx.ExecContext(
		ctx,
		query,
		snapshot.GoalID,
		snapshot.ScopeRoundID,
		snapshot.OwnerUserID,
		snapshot.RuntimeSessionKey,
		snapshot.SourceKind,
		snapshot.SourceID,
	)
	return err
}

func usageSourceGoalEvent(
	snapshot protocol.GoalUsageSourceSnapshot,
	item protocol.Goal,
	delta int64,
) protocol.GoalEvent {
	usage := (protocol.GoalUsage{
		ActualTotalTokens: delta,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}).NormalizeTotals()
	return protocol.GoalEvent{
		ID:         snapshot.EventID,
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    snapshot.RoundID,
		Payload: map[string]any{
			"usage": usage,
			"usage_source": map[string]any{
				"runtime_session_key": snapshot.RuntimeSessionKey,
				"source_kind":         snapshot.SourceKind,
				"source_id":           snapshot.SourceID,
				"scope_round_id":      snapshot.ScopeRoundID,
			},
		},
		CreatedAt: snapshot.ObservedAt,
	}
}

func usageSourceScopeClaimEvent(
	binding protocol.GoalUsageScopeBinding,
	item protocol.Goal,
	sourceRoundID string,
	usage protocol.GoalUsage,
	childPendingCount int64,
	parentPendingCount int64,
	parentUnavailableCount int64,
	attribution string,
) protocol.GoalEvent {
	return protocol.GoalEvent{
		ID:         binding.UsageEventID,
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    strings.TrimSpace(sourceRoundID),
		Payload: map[string]any{
			"usage": usage.NormalizeTotals(),
			"usage_source": map[string]any{
				"source_kind":              binding.SourceKind,
				"scope_round_id":           binding.ScopeRoundID,
				"pending_count":            childPendingCount + parentPendingCount,
				"child_pending_count":      childPendingCount,
				"parent_pending_count":     parentPendingCount,
				"parent_unavailable_count": parentUnavailableCount,
				"attribution":              attribution,
			},
		},
		CreatedAt: binding.BoundAt,
	}
}

func (r *Repository) getUsageSourceGoal(
	ctx context.Context,
	tx *sql.Tx,
	goalID string,
) (*protocol.Goal, error) {
	item, err := scanGoal(tx.QueryRowContext(ctx, goalSelectQuery("goal_id = "+r.bind(1)), goalID))
	if err != nil {
		return nil, err
	}
	return &item, nil
}
