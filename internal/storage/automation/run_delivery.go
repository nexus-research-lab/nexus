// INPUT: Automation run 的投递结果、retry/dead-letter 状态与 due 查询边界。
// OUTPUT: CAS-safe 投递观测写入、到期 retry rows 与最早 next-attempt deadline。
// POS: Automation delivery durable state；service timer 只消费其 deadline，不复制状态机。
package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// NextDeliveryRetryAt 返回最早可自动重试的失败投递。NULL deadline 使用
// updated_at，保持与 ListDueDeliveryRetries 相同的立即恢复语义。
func (r *Repository) NextDeliveryRetryAt(
	ctx context.Context,
	maxAttempts int,
) (*time.Time, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var deadline any
	err := r.db.QueryRowContext(ctx, `
SELECT MIN(COALESCE(delivery_next_attempt_at, updated_at))
FROM automation_task_runs
WHERE delivery_status = `+r.bind(1)+`
  AND delivery_dead_letter_at IS NULL
  AND delivery_attempts < `+r.bind(2),
		automationdomain.DeliveryStatusFailed,
		maxAttempts,
	).Scan(&deadline)
	if err != nil {
		return nil, err
	}
	return storage.NullableTime(deadline)
}

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
		`UPDATE automation_task_runs
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
func (r *Repository) ListDueDeliveryRetries(ctx context.Context, now time.Time, maxAttempts int, limit int) ([]automationdomain.ScheduledTaskRun, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs
WHERE delivery_status = ` + r.bind(1) + `
  AND delivery_dead_letter_at IS NULL
  AND delivery_attempts < ` + r.bind(2) + `
  AND (delivery_next_attempt_at IS NULL OR delivery_next_attempt_at <= ` + r.bind(3) + `)
ORDER BY COALESCE(delivery_next_attempt_at, updated_at), updated_at, run_id
LIMIT ` + r.bind(4)
	rows, err := r.db.QueryContext(ctx, query, automationdomain.DeliveryStatusFailed, maxAttempts, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]automationdomain.ScheduledTaskRun, 0)
	for rows.Next() {
		item, scanErr := scanScheduledTaskRun(rows)
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
	case "", automationdomain.DeliveryModeNone:
		return automationdomain.DeliveryStatusNotRequired
	default:
		return automationdomain.DeliveryStatusPending
	}
}

func finishedRunDeliveryStatus(input RunFinishInput) string {
	if deliveryStatus := strings.TrimSpace(input.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	switch strings.TrimSpace(input.Status) {
	case automationdomain.RunStatusPending, automationdomain.RunStatusRunning, automationdomain.RunStatusQueuedToMain:
		return automationdomain.DeliveryStatusPending
	case automationdomain.RunStatusSucceeded:
		return automationdomain.DeliveryStatusNotRequired
	case automationdomain.RunStatusFailed, automationdomain.RunStatusCancelled, automationdomain.RunStatusSkipped:
		return automationdomain.DeliveryStatusNotAttempted
	default:
		return automationdomain.DeliveryStatusNotAttempted
	}
}
