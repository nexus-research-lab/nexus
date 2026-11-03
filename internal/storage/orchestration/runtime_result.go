// INPUT: Host-authorized immutable result intake and domain reconciliation outcomes.
// OUTPUT: Durable once-per-Attempt/Submission receipts with bounded pending scans.
// POS: SQL intake/outcome boundary; it never interprets model text or applies work decisions.
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrRuntimeResultConflict = errors.New("runtime result conflicts with its immutable intake")

type RuntimeResultReceipt struct {
	ID, ExecutionID, OwnerUserID, Kind, TargetID string
	ActorJSON, ResultJSON, State, Reason         string
}

const runtimeResultSelect = `SELECT receipt_id, execution_id, owner_user_id, kind, target_id, actor_json, result_json, state, reason FROM execution_result_receipts `

func scanRuntimeResult(row interface{ Scan(...any) error }) (*RuntimeResultReceipt, error) {
	var value RuntimeResultReceipt
	err := row.Scan(&value.ID, &value.ExecutionID, &value.OwnerUserID, &value.Kind, &value.TargetID, &value.ActorJSON, &value.ResultJSON, &value.State, &value.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &value, err
}

func (r *Repository) GetRuntimeResultReceipt(ctx context.Context, id string) (*RuntimeResultReceipt, error) {
	return scanRuntimeResult(r.db.QueryRowContext(ctx, runtimeResultSelect+`WHERE receipt_id = `+r.bind(1), id))
}

// PutRuntimeResultReceipt commits intake before any domain mutation. Replays must
// preserve both the trusted actor and the explicit result, including round identity.
func (r *Repository) PutRuntimeResultReceipt(ctx context.Context, value RuntimeResultReceipt) (*RuntimeResultReceipt, error) {
	now := r.timestamp(r.currentTime())
	_, err := r.db.ExecContext(ctx, `INSERT INTO execution_result_receipts
(receipt_id, execution_id, owner_user_id, kind, target_id, actor_json, result_json, next_attempt_at, created_at, updated_at)
VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`)
ON CONFLICT (receipt_id) DO NOTHING`, value.ID, value.ExecutionID, value.OwnerUserID, value.Kind, value.TargetID, value.ActorJSON, value.ResultJSON, now, now, now)
	if err != nil {
		return nil, err
	}
	stored, err := r.GetRuntimeResultReceipt(ctx, value.ID)
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.ExecutionID != value.ExecutionID || stored.OwnerUserID != value.OwnerUserID || stored.Kind != value.Kind || stored.TargetID != value.TargetID || stored.ActorJSON != value.ActorJSON || stored.ResultJSON != value.ResultJSON {
		return nil, ErrRuntimeResultConflict
	}
	return stored, nil
}

func (r *Repository) ListPendingRuntimeResults(ctx context.Context, now time.Time, limit int) ([]RuntimeResultReceipt, error) {
	rows, err := r.db.QueryContext(ctx, runtimeResultSelect+`WHERE state = 'pending' AND next_attempt_at <= `+r.bind(1)+` ORDER BY next_attempt_at, receipt_id LIMIT `+r.bind(2), r.timestamp(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []RuntimeResultReceipt
	for rows.Next() {
		value, err := scanRuntimeResult(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *value)
	}
	return result, rows.Err()
}

func (r *Repository) SetRuntimeResultState(ctx context.Context, id, state, reason string, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE execution_result_receipts SET state = `+r.bind(1)+`, reason = `+r.bind(2)+`, next_attempt_at = `+r.bind(3)+`, updated_at = `+r.bind(4)+` WHERE receipt_id = `+r.bind(5)+` AND state = 'pending'`, state, reason, r.timestamp(next), r.timestamp(r.currentTime()), id)
	return err
}

func (r *Repository) HasPendingDeliveryResult(ctx context.Context, executionID, attemptID string) (bool, error) {
	var found int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM execution_result_receipts WHERE execution_id = `+r.bind(1)+` AND kind = 'delivery' AND target_id = `+r.bind(2)+` AND state = 'pending'`, executionID, attemptID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
