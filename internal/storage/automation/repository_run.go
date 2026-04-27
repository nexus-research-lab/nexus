package automation

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListRunsByJob 列出任务运行历史。
func (r *Repository) ListRunsByJob(ctx context.Context, jobID string) ([]protocol.CronRun, error) {
	query := `
SELECT
    run_id,
    job_id,
    status,
    scheduled_for,
    started_at,
    finished_at,
    attempts,
    error_message,
    created_at,
    updated_at
FROM automation_cron_runs
WHERE job_id = ` + r.bind(1) + `
ORDER BY created_at DESC, run_id DESC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(jobID))
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
func (r *Repository) InsertRunPending(ctx context.Context, runID string, jobID string, scheduledFor *time.Time) error {
	_, err := r.execWithRetry(ctx, r.insertRunPendingQuery, runID, jobID, protocol.RunStatusPending, scheduledFor, 0)
	return err
}

// MarkRunRunning 标记 run 开始执行。
func (r *Repository) MarkRunRunning(ctx context.Context, runID string, startedAt time.Time) error {
	_, err := r.execWithRetry(ctx, r.markRunRunningQuery, protocol.RunStatusRunning, startedAt.UTC(), runID)
	return err
}

// MarkRunFinished 标记 run 结束状态。
func (r *Repository) MarkRunFinished(ctx context.Context, runID string, status string, finishedAt time.Time, errorMessage *string) error {
	_, err := r.execWithRetry(ctx, r.markRunFinishedQuery, status, finishedAt.UTC(), nullableString(errorMessage), runID)
	return err
}
