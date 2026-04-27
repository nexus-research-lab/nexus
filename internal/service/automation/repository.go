package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type sqlRepository struct {
	db         *sql.DB
	isPostgres bool
}

const (
	automationWriteRetryAttempts = 4
	automationWriteRetryDelay    = 50 * time.Millisecond
)

// NewRepository 创建自动化仓储。
func NewRepository(cfg config.Config, db *sql.DB) *sqlRepository {
	return &sqlRepository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *sqlRepository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// ListCronJobs 列出定时任务。
func (r *sqlRepository) ListCronJobs(ctx context.Context, agentID string) ([]CronJob, error) {
	query := `
SELECT
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    enabled
FROM automation_cron_jobs`
	args := []any{}
	if strings.TrimSpace(agentID) != "" {
		query += " WHERE agent_id = " + r.bind(1)
		args = append(args, strings.TrimSpace(agentID))
	}
	query += " ORDER BY created_at DESC, job_id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CronJob, 0)
	for rows.Next() {
		item, scanErr := scanCronJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountEnabledCronJobs 统计启用中的定时任务数量。
func (r *sqlRepository) CountEnabledCronJobs(ctx context.Context, agentID string) (int, error) {
	query := "SELECT COUNT(1) FROM automation_cron_jobs WHERE enabled = " + r.bind(1)
	args := []any{true}
	if strings.TrimSpace(agentID) != "" {
		query += " AND agent_id = " + r.bind(2)
		args = append(args, strings.TrimSpace(agentID))
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetCronJob 读取单个任务。
func (r *sqlRepository) GetCronJob(ctx context.Context, jobID string) (*CronJob, error) {
	query := `
SELECT
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    enabled
FROM automation_cron_jobs
WHERE job_id = ` + r.bind(1)

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(jobID))
	item, err := scanCronJobRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpsertCronJob 创建或更新任务。
func (r *sqlRepository) UpsertCronJob(ctx context.Context, job CronJob) (*CronJob, error) {
	query := fmt.Sprintf(`
INSERT INTO automation_cron_jobs (
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    enabled,
    created_at,
    updated_at
) VALUES (
    %s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
)
ON CONFLICT(job_id) DO UPDATE SET
    name = EXCLUDED.name,
    agent_id = EXCLUDED.agent_id,
    schedule_kind = EXCLUDED.schedule_kind,
    run_at = EXCLUDED.run_at,
    interval_seconds = EXCLUDED.interval_seconds,
    cron_expression = EXCLUDED.cron_expression,
    timezone = EXCLUDED.timezone,
    instruction = EXCLUDED.instruction,
    session_target_kind = EXCLUDED.session_target_kind,
    bound_session_key = EXCLUDED.bound_session_key,
    named_session_key = EXCLUDED.named_session_key,
    wake_mode = EXCLUDED.wake_mode,
    delivery_mode = EXCLUDED.delivery_mode,
    delivery_channel = EXCLUDED.delivery_channel,
    delivery_to = EXCLUDED.delivery_to,
    delivery_account_id = EXCLUDED.delivery_account_id,
    delivery_thread_id = EXCLUDED.delivery_thread_id,
    source_kind = EXCLUDED.source_kind,
    source_creator_agent_id = EXCLUDED.source_creator_agent_id,
    source_context_type = EXCLUDED.source_context_type,
    source_context_id = EXCLUDED.source_context_id,
    source_context_label = EXCLUDED.source_context_label,
    source_session_key = EXCLUDED.source_session_key,
    source_session_label = EXCLUDED.source_session_label,
    enabled = EXCLUDED.enabled,
    updated_at = CURRENT_TIMESTAMP`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8), r.bind(9), r.bind(10),
		r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15), r.bind(16), r.bind(17), r.bind(18), r.bind(19), r.bind(20),
		r.bind(21), r.bind(22), r.bind(23), r.bind(24), r.bind(25), r.bind(26),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		job.JobID,
		job.Name,
		job.AgentID,
		job.Schedule.Kind,
		nullStringPointer(job.Schedule.RunAt),
		nullIntPointer(job.Schedule.IntervalSeconds),
		nullStringPointer(job.Schedule.CronExpression),
		job.Schedule.Timezone,
		job.Instruction,
		job.SessionTarget.Kind,
		nullString(job.SessionTarget.BoundSessionKey),
		nullString(job.SessionTarget.NamedSessionKey),
		job.SessionTarget.WakeMode,
		job.Delivery.Mode,
		nullString(job.Delivery.Channel),
		nullString(job.Delivery.To),
		nullString(job.Delivery.AccountID),
		nullString(job.Delivery.ThreadID),
		job.Source.Kind,
		nullString(job.Source.CreatorAgentID),
		nullString(job.Source.ContextType),
		nullString(job.Source.ContextID),
		nullString(job.Source.ContextLabel),
		nullString(job.Source.SessionKey),
		nullString(job.Source.SessionLabel),
		job.Enabled,
	)
	if err != nil {
		return nil, err
	}
	return r.GetCronJob(ctx, job.JobID)
}

// DeleteCronJob 删除任务。
func (r *sqlRepository) DeleteCronJob(ctx context.Context, jobID string) error {
	_, err := r.execWithRetry(ctx, "DELETE FROM automation_cron_jobs WHERE job_id = "+r.bind(1), strings.TrimSpace(jobID))
	return err
}

// ListRunsByJob 列出任务运行历史。
func (r *sqlRepository) ListRunsByJob(ctx context.Context, jobID string) ([]CronRun, error) {
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

	items := make([]CronRun, 0)
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
func (r *sqlRepository) InsertRunPending(ctx context.Context, runID string, jobID string, scheduledFor *time.Time) error {
	query := fmt.Sprintf(`
INSERT INTO automation_cron_runs (
    run_id,
    job_id,
    status,
    scheduled_for,
    attempts,
    created_at,
    updated_at
) VALUES (%s,%s,%s,%s,%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
	)
	_, err := r.execWithRetry(ctx, query, runID, jobID, RunStatusPending, scheduledFor, 0)
	return err
}

// MarkRunRunning 标记 run 开始执行。
func (r *sqlRepository) MarkRunRunning(ctx context.Context, runID string, startedAt time.Time) error {
	query := fmt.Sprintf(`
UPDATE automation_cron_runs
SET status = %s,
    started_at = %s,
    attempts = attempts + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s`,
		r.bind(1), r.bind(2), r.bind(3),
	)
	_, err := r.execWithRetry(ctx, query, RunStatusRunning, startedAt.UTC(), runID)
	return err
}

// MarkRunFinished 标记 run 结束状态。
func (r *sqlRepository) MarkRunFinished(ctx context.Context, runID string, status string, finishedAt time.Time, errorMessage *string) error {
	query := fmt.Sprintf(`
UPDATE automation_cron_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4),
	)
	_, err := r.execWithRetry(ctx, query, status, finishedAt.UTC(), nullableString(errorMessage), runID)
	return err
}

// GetHeartbeatState 读取 heartbeat 配置。
func (r *sqlRepository) GetHeartbeatState(ctx context.Context, agentID string) (*HeartbeatConfig, *time.Time, *time.Time, error) {
	query := `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    last_heartbeat_at,
    last_ack_at
FROM automation_heartbeat_states
WHERE agent_id = ` + r.bind(1)
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(agentID))
	var (
		item          HeartbeatConfig
		lastHeartbeat sql.NullTime
		lastAck       sql.NullTime
	)
	err := row.Scan(
		&item.AgentID,
		&item.Enabled,
		&item.EverySeconds,
		&item.TargetMode,
		&item.AckMaxChars,
		&lastHeartbeat,
		&lastAck,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return &item, nullTimePointer(lastHeartbeat), nullTimePointer(lastAck), nil
}

// ListEnabledHeartbeatStates 列出已启用 heartbeat。
func (r *sqlRepository) ListEnabledHeartbeatStates(ctx context.Context) ([]HeartbeatConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars
FROM automation_heartbeat_states
WHERE enabled = TRUE
ORDER BY agent_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]HeartbeatConfig, 0)
	for rows.Next() {
		var item HeartbeatConfig
		if scanErr := rows.Scan(
			&item.AgentID,
			&item.Enabled,
			&item.EverySeconds,
			&item.TargetMode,
			&item.AckMaxChars,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpsertHeartbeatState 创建或更新 heartbeat 配置。
func (r *sqlRepository) UpsertHeartbeatState(ctx context.Context, stateID string, config HeartbeatConfig, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
	query := fmt.Sprintf(`
INSERT INTO automation_heartbeat_states (
    state_id,
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    last_heartbeat_at,
    last_ack_at,
    created_at,
    updated_at
) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(agent_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    every_seconds = EXCLUDED.every_seconds,
    target_mode = EXCLUDED.target_mode,
    ack_max_chars = EXCLUDED.ack_max_chars,
    last_heartbeat_at = EXCLUDED.last_heartbeat_at,
    last_ack_at = EXCLUDED.last_ack_at,
    updated_at = CURRENT_TIMESTAMP`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		stateID,
		config.AgentID,
		config.Enabled,
		config.EverySeconds,
		config.TargetMode,
		config.AckMaxChars,
		lastHeartbeatAt,
		lastAckAt,
	)
	return err
}

// InsertSystemEvent 写入系统事件。
func (r *sqlRepository) InsertSystemEvent(ctx context.Context, eventID string, eventType string, sourceType string, sourceID string, payload map[string]any) error {
	query := fmt.Sprintf(`
INSERT INTO automation_system_events (
    event_id,
    event_type,
    source_type,
    source_id,
    payload,
    status,
    created_at,
    updated_at
) VALUES (%s,%s,%s,%s,%s,%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = r.execWithRetry(ctx, query, eventID, eventType, sourceType, sourceID, string(body), "new")
	return err
}

// ListNewSystemEventsByAgent 列出指定 agent 尚未消费的系统事件。
func (r *sqlRepository) ListNewSystemEventsByAgent(ctx context.Context, agentID string) ([]SystemEvent, error) {
	query := `
SELECT
    event_id,
    event_type,
    source_type,
    source_id,
    payload,
    status,
    created_at
FROM automation_system_events
WHERE status = 'new'`
	if r.isPostgres {
		query += ` AND (source_id = ` + r.bind(1) + ` OR payload::jsonb->>'agent_id' = ` + r.bind(2) + `)`
	} else {
		query += ` AND (source_id = ` + r.bind(1) + ` OR json_extract(payload, '$.agent_id') = ` + r.bind(2) + `)`
	}
	query += `
ORDER BY created_at ASC, event_id ASC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(agentID), strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SystemEvent, 0)
	for rows.Next() {
		var item SystemEvent
		if scanErr := rows.Scan(
			&item.EventID,
			&item.EventType,
			&item.SourceType,
			&item.SourceID,
			&item.Payload,
			&item.Status,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkSystemEventStatus 更新系统事件状态。
func (r *sqlRepository) MarkSystemEventStatus(ctx context.Context, eventID string, status string) error {
	query := fmt.Sprintf(`
UPDATE automation_system_events
SET status = %s,
    processed_at = CASE WHEN %s IN ('processed', 'failed') THEN CURRENT_TIMESTAMP ELSE processed_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE event_id = %s`,
		r.bind(1), r.bind(2), r.bind(3),
	)
	_, err := r.execWithRetry(ctx, query, status, status, eventID)
	return err
}

func (r *sqlRepository) execWithRetry(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r.isPostgres {
		return r.db.ExecContext(ctx, query, args...)
	}
	var lastErr error
	for attempt := 0; attempt < automationWriteRetryAttempts; attempt++ {
		result, err := r.db.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}
		if !isSQLiteLockedError(err) || attempt == automationWriteRetryAttempts-1 {
			return nil, err
		}
		lastErr = err
		delay := automationWriteRetryDelay * time.Duration(attempt+1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "database is locked")
}

func scanCronJob(scanner interface {
	Scan(dest ...any) error
}) (CronJob, error) {
	var (
		item               CronJob
		runAt              sql.NullString
		intervalSeconds    sql.NullInt64
		cronExpression     sql.NullString
		boundSessionKey    sql.NullString
		namedSessionKey    sql.NullString
		deliveryChannel    sql.NullString
		deliveryTo         sql.NullString
		deliveryAccountID  sql.NullString
		deliveryThreadID   sql.NullString
		sourceKind         sql.NullString
		sourceCreatorID    sql.NullString
		sourceContextType  sql.NullString
		sourceContextID    sql.NullString
		sourceContextLabel sql.NullString
		sourceSessionKey   sql.NullString
		sourceSessionLabel sql.NullString
	)
	err := scanner.Scan(
		&item.JobID,
		&item.Name,
		&item.AgentID,
		&item.Schedule.Kind,
		&runAt,
		&intervalSeconds,
		&cronExpression,
		&item.Schedule.Timezone,
		&item.Instruction,
		&item.SessionTarget.Kind,
		&boundSessionKey,
		&namedSessionKey,
		&item.SessionTarget.WakeMode,
		&item.Delivery.Mode,
		&deliveryChannel,
		&deliveryTo,
		&deliveryAccountID,
		&deliveryThreadID,
		&sourceKind,
		&sourceCreatorID,
		&sourceContextType,
		&sourceContextID,
		&sourceContextLabel,
		&sourceSessionKey,
		&sourceSessionLabel,
		&item.Enabled,
	)
	if err != nil {
		return CronJob{}, err
	}
	item.Schedule.RunAt = nullStringToPointer(runAt)
	item.Schedule.IntervalSeconds = nullIntToPointer(intervalSeconds)
	item.Schedule.CronExpression = nullStringToPointer(cronExpression)
	item.SessionTarget.BoundSessionKey = nullStringValue(boundSessionKey)
	item.SessionTarget.NamedSessionKey = nullStringValue(namedSessionKey)
	item.Delivery.Channel = nullStringValue(deliveryChannel)
	item.Delivery.To = nullStringValue(deliveryTo)
	item.Delivery.AccountID = nullStringValue(deliveryAccountID)
	item.Delivery.ThreadID = nullStringValue(deliveryThreadID)
	item.Source.Kind = nullStringValue(sourceKind)
	item.Source.CreatorAgentID = nullStringValue(sourceCreatorID)
	item.Source.ContextType = nullStringValue(sourceContextType)
	item.Source.ContextID = nullStringValue(sourceContextID)
	item.Source.ContextLabel = nullStringValue(sourceContextLabel)
	item.Source.SessionKey = nullStringValue(sourceSessionKey)
	item.Source.SessionLabel = nullStringValue(sourceSessionLabel)
	item.Source = item.Source.Normalized()
	return item, nil
}

func scanCronJobRow(row *sql.Row) (*CronJob, error) {
	item, err := scanCronJob(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanCronRun(scanner interface {
	Scan(dest ...any) error
}) (CronRun, error) {
	var (
		item         CronRun
		scheduledFor sql.NullTime
		startedAt    sql.NullTime
		finishedAt   sql.NullTime
		errorMessage sql.NullString
	)
	err := scanner.Scan(
		&item.RunID,
		&item.JobID,
		&item.Status,
		&scheduledFor,
		&startedAt,
		&finishedAt,
		&item.Attempts,
		&errorMessage,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return CronRun{}, err
	}
	item.ScheduledFor = nullTimePointer(scheduledFor)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	item.ErrorMessage = nullStringToPointer(errorMessage)
	return item, nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullableString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func nullStringPointer(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func nullIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func nullStringToPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := strings.TrimSpace(value.String)
	return &result
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullIntToPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}
