// INPUT: Goal continuation plan, optimistic Goal version and durable launch CAS.
// OUTPUT: atomically reserved, claimed, retried, settled or released launch receipts.
// POS: Goal continuation crash-recovery transaction boundary; prompt never enters Goal metadata.
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

func (r *Repository) ReserveGoalContinuation(ctx context.Context, goal protocol.Goal, expectedVersion int64, event protocol.GoalEvent, plan protocol.GoalContinuationPlan) (*protocol.Goal, error) {
	if goal.ID == "" || plan.RoundID == "" || plan.GoalID != goal.ID || plan.SessionKey != goal.SessionKey ||
		plan.ObjectiveRevision != goal.ObjectiveRevision() || plan.Status != protocol.GoalContinuationPlanStatusScheduled ||
		plan.NextAttemptAt == nil || plan.Version <= 0 || strings.TrimSpace(plan.Prompt) == "" || strings.TrimSpace(plan.Purpose) == "" {
		return nil, fmt.Errorf("goal continuation reservation identity is invalid")
	}
	if err := validateGoalMutationEvent(goal, normalizeGoalCreateEvent(event)); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = r.updateGoal(ctx, tx, goal, expectedVersion); err != nil {
		return nil, err
	}
	if err = r.cancelGoalContinuations(ctx, tx, goal.ID, goal.ObjectiveRevision(), "Goal objective revision advanced", goal.UpdatedAt); err != nil {
		return nil, err
	}
	if err = r.insertGoalEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if err = r.insertGoalContinuationPlan(ctx, tx, plan); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &goal, nil
}

func (r *Repository) insertGoalContinuationPlan(ctx context.Context, executor goalEventExecutor, plan protocol.GoalContinuationPlan) error {
	metadataValue := marshalStringMap(plan.Metadata)
	query := fmt.Sprintf(`INSERT INTO goal_continuation_plans (
    round_id, goal_id, session_key, objective_revision, execution_id,
    previous_round_id, prompt, purpose, metadata_json, status, version,
    attempt_count, next_attempt_at, claim_expires_at, last_error,
    created_at, updated_at, settled_at
) VALUES (%s)`, r.bindList(18))
	_, err := executor.ExecContext(ctx, query,
		strings.TrimSpace(plan.RoundID), strings.TrimSpace(plan.GoalID), strings.TrimSpace(plan.SessionKey), plan.ObjectiveRevision,
		nullString(plan.ExecutionID), nullString(plan.PreviousRoundID), strings.TrimSpace(plan.Prompt), strings.TrimSpace(plan.Purpose),
		metadataValue, plan.Status, plan.Version, plan.AttemptCount, nullableTime(plan.NextAttemptAt),
		nullableTime(plan.ClaimExpiresAt), nullString(plan.LastError), plan.CreatedAt.UTC(), plan.UpdatedAt.UTC(), nullableTime(plan.SettledAt))
	return err
}

func (r *Repository) GetOpenGoalContinuation(ctx context.Context, goalID string, objectiveRevision int64) (*protocol.GoalContinuationPlan, error) {
	query := goalContinuationPlanSelectQuery("goal_id = " + r.bind(1) + " AND objective_revision = " + r.bind(2) + " AND status IN ('scheduled', 'claimed', 'started')")
	plan, err := scanGoalContinuationPlan(r.db.QueryRowContext(ctx, query, strings.TrimSpace(goalID), objectiveRevision))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *Repository) ClaimGoalContinuation(ctx context.Context, roundID string, now, claimExpiresAt time.Time) (*protocol.GoalContinuationPlan, error) {
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
SET status = 'claimed', version = version + 1, attempt_count = attempt_count + 1,
    next_attempt_at = NULL, claim_expires_at = %s, last_error = NULL, updated_at = %s
WHERE round_id = %s
  AND ((status = 'scheduled' AND next_attempt_at <= %s)
    OR (status IN ('claimed', 'started') AND claim_expires_at <= %s))`, r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5))
	result, err := r.db.ExecContext(ctx, query, claimExpiresAt.UTC(), now.UTC(), strings.TrimSpace(roundID), now.UTC(), now.UTC())
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	return r.getGoalContinuationPlan(ctx, roundID)
}

func (r *Repository) MarkGoalContinuationStarted(ctx context.Context, roundID string, now, recoveryAt time.Time) error {
	if !recoveryAt.After(now) {
		return fmt.Errorf("goal continuation started recovery deadline is invalid")
	}
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
	SET status = 'started', version = version + 1, next_attempt_at = NULL,
	    claim_expires_at = %s, last_error = NULL, updated_at = %s, settled_at = NULL
WHERE round_id = %s AND status = 'claimed'`, r.bind(1), r.bind(2), r.bind(3))
	result, err := r.db.ExecContext(ctx, query, recoveryAt.UTC(), now.UTC(), strings.TrimSpace(roundID))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected > 0 {
		return err
	}
	plan, err := r.getGoalContinuationPlan(ctx, roundID)
	if err == nil && plan != nil && plan.Status == protocol.GoalContinuationPlanStatusStarted {
		return nil
	}
	if err != nil {
		return err
	}
	return sql.ErrNoRows
}

func (r *Repository) SettleGoalContinuation(ctx context.Context, goalID, roundID string, objectiveRevision int64, now time.Time) error {
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
SET status = 'settled', version = version + 1, next_attempt_at = NULL,
    claim_expires_at = NULL, last_error = NULL, updated_at = %s, settled_at = %s
WHERE round_id = %s AND goal_id = %s AND objective_revision = %s
  AND status IN ('claimed', 'started')`, r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5))
	result, err := r.db.ExecContext(ctx, query, now.UTC(), now.UTC(), strings.TrimSpace(roundID), strings.TrimSpace(goalID), objectiveRevision)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected > 0 {
		return err
	}
	plan, err := r.getGoalContinuationPlan(ctx, roundID)
	if err == nil && plan != nil &&
		plan.Status == protocol.GoalContinuationPlanStatusSettled &&
		plan.GoalID == strings.TrimSpace(goalID) &&
		plan.ObjectiveRevision == objectiveRevision {
		return nil
	}
	if err != nil {
		return err
	}
	return sql.ErrNoRows
}

func (r *Repository) RetryGoalContinuation(ctx context.Context, roundID, reason string, nextAttemptAt, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(roundID) == "" || reason == "" || !nextAttemptAt.After(now) {
		return fmt.Errorf("goal continuation retry identity is invalid")
	}
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
SET status = 'scheduled', version = version + 1, next_attempt_at = %s,
    claim_expires_at = NULL, last_error = %s, updated_at = %s
WHERE round_id = %s AND status = 'claimed'`, r.bind(1), r.bind(2), r.bind(3), r.bind(4))
	result, err := r.db.ExecContext(ctx, query, nextAttemptAt.UTC(), nullString(reason), now.UTC(), strings.TrimSpace(roundID))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *Repository) ReleaseGoalContinuation(ctx context.Context, goal protocol.Goal, expectedVersion int64, event protocol.GoalEvent, roundID string, now time.Time) (*protocol.Goal, error) {
	if goal.ID == "" || strings.TrimSpace(roundID) == "" || event.GoalID != goal.ID || event.SessionKey != goal.SessionKey || event.EventType != "continuation_deferred" || goal.Version != expectedVersion+1 {
		return nil, fmt.Errorf("goal continuation release identity is invalid")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
SET status = 'released', version = version + 1, next_attempt_at = NULL,
    claim_expires_at = NULL, last_error = %s, updated_at = %s, settled_at = %s
WHERE round_id = %s AND status IN ('scheduled', 'claimed')`, r.bind(1), r.bind(2), r.bind(3), r.bind(4))
	result, err := tx.ExecContext(ctx, query, nullString(stringValue(event.Payload["reason"])), now.UTC(), now.UTC(), strings.TrimSpace(roundID))
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	if err = r.updateGoal(ctx, tx, goal, expectedVersion); err != nil {
		return nil, err
	}
	if err = r.insertGoalEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &goal, nil
}

func (r *Repository) cancelGoalContinuations(ctx context.Context, executor goalEventExecutor, goalID string, keepRevision int64, reason string, now time.Time) error {
	if strings.TrimSpace(goalID) == "" {
		return fmt.Errorf("goal continuation cancellation identity is invalid")
	}
	predicate := "goal_id = " + r.bind(4) + " AND status IN ('scheduled', 'claimed', 'started')"
	args := []any{nullString(reason), now.UTC(), now.UTC(), strings.TrimSpace(goalID)}
	if keepRevision > 0 {
		predicate += " AND objective_revision <> " + r.bind(5)
		args = append(args, keepRevision)
	}
	query := fmt.Sprintf(`UPDATE goal_continuation_plans
SET status = 'cancelled', version = version + 1, next_attempt_at = NULL,
    claim_expires_at = NULL, last_error = %s, updated_at = %s, settled_at = %s
WHERE %s`, r.bind(1), r.bind(2), r.bind(3), predicate)
	_, err := executor.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) getGoalContinuationPlan(ctx context.Context, roundID string) (*protocol.GoalContinuationPlan, error) {
	plan, err := scanGoalContinuationPlan(r.db.QueryRowContext(ctx, goalContinuationPlanSelectQuery("round_id = "+r.bind(1)), strings.TrimSpace(roundID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func goalContinuationPlanSelectQuery(where string) string {
	return `SELECT round_id, goal_id, session_key, objective_revision,
    execution_id, previous_round_id, prompt, purpose, metadata_json,
    status, version, attempt_count, next_attempt_at, claim_expires_at,
    last_error, created_at, updated_at, settled_at
FROM goal_continuation_plans WHERE ` + where
}

func scanGoalContinuationPlan(scanner interface{ Scan(...any) error }) (protocol.GoalContinuationPlan, error) {
	var plan protocol.GoalContinuationPlan
	var status, metadataJSON string
	var executionID, previousRoundID, lastError sql.NullString
	var nextAttemptAt, claimExpiresAt, settledAt sql.NullTime
	err := scanner.Scan(&plan.RoundID, &plan.GoalID, &plan.SessionKey, &plan.ObjectiveRevision,
		&executionID, &previousRoundID, &plan.Prompt, &plan.Purpose, &metadataJSON,
		&status, &plan.Version, &plan.AttemptCount, &nextAttemptAt, &claimExpiresAt,
		&lastError, &plan.CreatedAt, &plan.UpdatedAt, &settledAt)
	if err != nil {
		return protocol.GoalContinuationPlan{}, err
	}
	plan.ExecutionID, plan.PreviousRoundID, plan.LastError = nullStringValue(executionID), nullStringValue(previousRoundID), nullStringValue(lastError)
	plan.NextAttemptAt, plan.ClaimExpiresAt, plan.SettledAt = nullTimePointer(nextAttemptAt), nullTimePointer(claimExpiresAt), nullTimePointer(settledAt)
	plan.Status, plan.Metadata = protocol.GoalContinuationPlanStatus(status), parseStringMap(metadataJSON)
	return plan, nil
}

func marshalStringMap(input map[string]string) string {
	values := make(map[string]any, len(input))
	for key, value := range input {
		values[key] = value
	}
	return marshalMap(values)
}

func parseStringMap(raw string) map[string]string {
	parsed, result := parseMap(raw), map[string]string{}
	for key, value := range parsed {
		if text, ok := value.(string); ok {
			result[key] = text
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func stringValue(value any) string { text, _ := value.(string); return strings.TrimSpace(text) }
