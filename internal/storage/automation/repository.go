package automation

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 封装自动化任务与 heartbeat 的 SQL 读写。
type Repository struct {
	db                         *sql.DB
	isPostgres                 bool
	upsertCronJobQuery         string
	insertRunPendingQuery      string
	markRunRunningQuery        string
	markRunFinishedQuery       string
	upsertHeartbeatStateQuery  string
	insertSystemEventQuery     string
	markSystemEventStatusQuery string
}

const upsertCronJobQueryTemplate = `
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
    %s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
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
    updated_at = CURRENT_TIMESTAMP`

// NewRepository 创建自动化仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	repository := &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
	repository.upsertCronJobQuery = fmt.Sprintf(upsertCronJobQueryTemplate, repository.bindList(26))
	repository.insertRunPendingQuery = fmt.Sprintf(
		`INSERT INTO automation_cron_runs (
    run_id,
    job_id,
    status,
    scheduled_for,
    attempts,
    created_at,
    updated_at
) VALUES (%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		repository.bindList(5),
	)
	repository.markRunRunningQuery = fmt.Sprintf(
		`UPDATE automation_cron_runs
SET status = %s,
    started_at = %s,
    attempts = attempts + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s`,
		repository.bind(1), repository.bind(2), repository.bind(3),
	)
	repository.markRunFinishedQuery = fmt.Sprintf(
		`UPDATE automation_cron_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s`,
		repository.bind(1), repository.bind(2), repository.bind(3), repository.bind(4),
	)
	repository.upsertHeartbeatStateQuery = fmt.Sprintf(
		`INSERT INTO automation_heartbeat_states (
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
) VALUES (%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(agent_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    every_seconds = EXCLUDED.every_seconds,
    target_mode = EXCLUDED.target_mode,
    ack_max_chars = EXCLUDED.ack_max_chars,
    last_heartbeat_at = EXCLUDED.last_heartbeat_at,
    last_ack_at = EXCLUDED.last_ack_at,
    updated_at = CURRENT_TIMESTAMP`,
		repository.bindList(8),
	)
	repository.insertSystemEventQuery = fmt.Sprintf(
		`INSERT INTO automation_system_events (
    event_id,
    event_type,
    source_type,
    source_id,
    payload,
    status,
    created_at,
    updated_at
) VALUES (%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		repository.bindList(6),
	)
	repository.markSystemEventStatusQuery = fmt.Sprintf(
		`UPDATE automation_system_events
SET status = %s,
    processed_at = CASE WHEN %s IN ('processed', 'failed') THEN CURRENT_TIMESTAMP ELSE processed_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE event_id = %s`,
		repository.bind(1), repository.bind(2), repository.bind(3),
	)
	return repository
}

func (r *Repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (r *Repository) bindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, r.bind(index))
	}
	return strings.Join(items, ",")
}
