package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RunDeliveryUpdateInput 表示单独刷新 run 投递状态的输入。
type RunDeliveryUpdateInput struct {
	RunID                 string
	DeliveryMode          string
	DeliveryTo            string
	DeliveryStatus        string
	DeliveryError         *string
	DeliveredAt           *time.Time
	DeliveryAttempted     bool
	DeliveryNextAttemptAt *time.Time
	DeliveryDeadLetterAt  *time.Time
}

// MarkRunDelivery 更新 run 的投递状态和投递观测信息。
func (r *Repository) MarkRunDelivery(ctx context.Context, input RunDeliveryUpdateInput) error {
	query := fmt.Sprintf(
		`UPDATE automation_cron_runs
SET delivery_mode = COALESCE(%s, delivery_mode),
    delivery_to = COALESCE(%s, delivery_to),
    delivery_status = %s,
    delivery_error = %s,
    delivered_at = %s,
    delivery_attempts = delivery_attempts + CASE WHEN %s THEN 1 ELSE 0 END,
    delivery_next_attempt_at = %s,
    delivery_dead_letter_at = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
		r.bind(5),
		r.bind(6),
		r.bind(7),
		r.bind(8),
		r.bind(9),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		nullString(strings.TrimSpace(input.DeliveryMode)),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(strings.TrimSpace(input.DeliveryStatus)),
		nullableString(input.DeliveryError),
		nullableTime(input.DeliveredAt),
		input.DeliveryAttempted,
		nullableTime(input.DeliveryNextAttemptAt),
		nullableTime(input.DeliveryDeadLetterAt),
		strings.TrimSpace(input.RunID),
	)
	return err
}

// ListDueDeliveryRetries 列出到期的失败投递 run。
func (r *Repository) ListDueDeliveryRetries(ctx context.Context, now time.Time, maxAttempts int, limit int) ([]protocol.CronRun, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if limit <= 0 {
		limit = 20
	}
	query := `
SELECT
    run_id,
    job_id,
    owner_user_id,
    status,
    trigger_kind,
    session_key,
    round_id,
    session_id,
    message_count,
    delivery_mode,
    delivery_to,
    delivery_status,
    delivery_error,
    delivered_at,
    delivery_attempts,
    delivery_next_attempt_at,
    delivery_dead_letter_at,
    scheduled_for,
    started_at,
    finished_at,
    attempts,
    error_message,
    result_summary,
    assistant_text,
    result_text,
    artifact_path,
    created_at,
    updated_at
FROM automation_cron_runs
WHERE delivery_status = ` + r.bind(1) + `
  AND delivery_dead_letter_at IS NULL
  AND delivery_attempts < ` + r.bind(2) + `
  AND (delivery_next_attempt_at IS NULL OR delivery_next_attempt_at <= ` + r.bind(3) + `)
ORDER BY COALESCE(delivery_next_attempt_at, updated_at), updated_at, run_id
LIMIT ` + r.bind(4)
	rows, err := r.db.QueryContext(ctx, query, protocol.DeliveryStatusFailed, maxAttempts, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]protocol.CronRun, 0)
	for rows.Next() {
		item, scanErr := scanCronRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func initialRunDeliveryStatus(input RunPendingInput) string {
	if deliveryStatus := strings.TrimSpace(input.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	switch strings.TrimSpace(input.DeliveryMode) {
	case "", protocol.DeliveryModeNone:
		return protocol.DeliveryStatusNotRequired
	default:
		return protocol.DeliveryStatusPending
	}
}

func finishedRunDeliveryStatus(input RunFinishInput) string {
	if deliveryStatus := strings.TrimSpace(input.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	switch strings.TrimSpace(input.Status) {
	case protocol.RunStatusPending, protocol.RunStatusRunning:
		return protocol.DeliveryStatusPending
	case protocol.RunStatusSucceeded, protocol.RunStatusQueuedToMain:
		return protocol.DeliveryStatusNotRequired
	case protocol.RunStatusFailed, protocol.RunStatusCancelled, protocol.RunStatusSkipped:
		return protocol.DeliveryStatusNotAttempted
	default:
		return protocol.DeliveryStatusNotAttempted
	}
}
