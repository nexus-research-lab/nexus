package automation

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RunPendingInput 表示创建 run ledger 的输入。
type RunPendingInput struct {
	RunID        string
	JobID        string
	OwnerUserID  string
	ScheduledFor *time.Time
	TriggerKind  string
	SessionKey   string
	RoundID      string
	DeliveryMode string
	DeliveryTo   string
	Status       string
}

// RunFinishInput 表示结束 run ledger 的输入。
type RunFinishInput struct {
	RunID         string
	Status        string
	FinishedAt    time.Time
	ErrorMessage  *string
	SessionID     *string
	MessageCount  int
	ResultSummary *string
}

// ListRunsByJob 列出任务运行历史。ownerUserID 为空时表示全局作用域。
func (r *Repository) ListRunsByJob(ctx context.Context, ownerUserID string, jobID string) ([]protocol.CronRun, error) {
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
    scheduled_for,
    started_at,
    finished_at,
    attempts,
    error_message,
    result_summary,
    created_at,
    updated_at
FROM automation_cron_runs
WHERE job_id = ` + r.bind(1)
	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	query += `
ORDER BY created_at DESC, run_id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
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

// InsertRunPending 新建一条待执行 run。
func (r *Repository) InsertRunPending(ctx context.Context, input RunPendingInput) error {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = protocol.RunStatusPending
	}
	_, err := r.execWithRetry(
		ctx,
		r.insertRunPendingQuery,
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		status,
		strings.TrimSpace(input.TriggerKind),
		nullString(strings.TrimSpace(input.SessionKey)),
		nullString(strings.TrimSpace(input.RoundID)),
		nullString(strings.TrimSpace(input.DeliveryMode)),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		input.ScheduledFor,
		0,
	)
	return err
}

// MarkRunRunning 标记 run 开始执行。
func (r *Repository) MarkRunRunning(ctx context.Context, runID string, startedAt time.Time) error {
	_, err := r.execWithRetry(ctx, r.markRunRunningQuery, protocol.RunStatusRunning, startedAt.UTC(), runID)
	return err
}

// MarkRunFinished 标记 run 结束状态。
func (r *Repository) MarkRunFinished(ctx context.Context, input RunFinishInput) error {
	_, err := r.execWithRetry(
		ctx,
		r.markRunFinishedQuery,
		strings.TrimSpace(input.Status),
		input.FinishedAt.UTC(),
		nullableString(input.ErrorMessage),
		nullableString(input.SessionID),
		input.MessageCount,
		nullableString(input.ResultSummary),
		strings.TrimSpace(input.RunID),
	)
	return err
}
