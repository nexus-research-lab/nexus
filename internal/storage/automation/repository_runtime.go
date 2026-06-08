package automation

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// JobRuntimeUpdateInput 表示定时任务调度运行态的持久化快照。
type JobRuntimeUpdateInput struct {
	JobID              string
	NextRunAt          *time.Time
	RunningRunID       string
	RunningStartedAt   *time.Time
	LastRunAt          *time.Time
	LastRunStatus      string
	FailureStreak      int
	LastError          *string
	LastDeliveryStatus string
}

// JobRuntimeClaimInput 表示一次调度执行的领取请求。
type JobRuntimeClaimInput struct {
	JobID         string
	RunID         string
	StartedAt     time.Time
	NextRunAt     *time.Time
	OverlapPolicy string
	AllowDisabled bool
}

// ClaimCronJobRuntime 通过条件更新领取一次任务执行权。
func (r *Repository) ClaimCronJobRuntime(ctx context.Context, input JobRuntimeClaimInput) (bool, error) {
	args := []any{
		nullString(input.RunID),
		input.StartedAt.UTC(),
		nullableTime(input.NextRunAt),
		strings.TrimSpace(input.JobID),
	}
	query := fmt.Sprintf(
		`UPDATE automation_cron_jobs
SET running_run_id = %s,
    running_started_at = %s,
    next_run_at = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
	)
	if !input.AllowDisabled {
		args = append(args, true)
		query += fmt.Sprintf(" AND enabled = %s", r.bind(len(args)))
	}
	if strings.TrimSpace(input.OverlapPolicy) == "skip" {
		query += " AND (running_run_id IS NULL OR running_run_id = '')"
	}
	result, err := r.execWithRetry(ctx, query, args...)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateCronJobRuntime 写入调度器运行态，不覆盖任务定义字段。
func (r *Repository) UpdateCronJobRuntime(ctx context.Context, input JobRuntimeUpdateInput) error {
	query := fmt.Sprintf(
		`UPDATE automation_cron_jobs
SET next_run_at = %s,
    running_run_id = %s,
    running_started_at = %s,
    last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s`,
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
		nullableTime(input.NextRunAt),
		nullString(input.RunningRunID),
		nullableTime(input.RunningStartedAt),
		nullableTime(input.LastRunAt),
		nullString(input.LastRunStatus),
		input.FailureStreak,
		nullableString(input.LastError),
		nullString(input.LastDeliveryStatus),
		strings.TrimSpace(input.JobID),
	)
	return err
}
