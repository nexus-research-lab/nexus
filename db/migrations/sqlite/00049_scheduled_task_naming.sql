-- +goose Up
ALTER TABLE automation_cron_jobs RENAME TO automation_scheduled_tasks;
ALTER TABLE automation_cron_runs RENAME TO automation_task_runs;

DROP INDEX IF EXISTS idx_automation_cron_jobs_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_due;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_running;
DROP INDEX IF EXISTS idx_automation_cron_jobs_expires_at;
DROP INDEX IF EXISTS idx_automation_cron_runs_job;
DROP INDEX IF EXISTS idx_automation_cron_runs_status;
DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_owner_job_created;

CREATE INDEX idx_automation_scheduled_tasks_agent ON automation_scheduled_tasks (agent_id);
CREATE INDEX idx_automation_scheduled_tasks_created ON automation_scheduled_tasks (created_at DESC, job_id DESC);
CREATE INDEX idx_automation_scheduled_tasks_agent_created ON automation_scheduled_tasks (agent_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_scheduled_tasks_enabled_agent ON automation_scheduled_tasks (enabled, agent_id);
CREATE INDEX idx_automation_scheduled_tasks_owner_created ON automation_scheduled_tasks (owner_user_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_scheduled_tasks_owner_agent_created ON automation_scheduled_tasks (owner_user_id, agent_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_scheduled_tasks_owner_enabled_agent ON automation_scheduled_tasks (owner_user_id, enabled, agent_id);
CREATE INDEX idx_automation_scheduled_tasks_runtime_due ON automation_scheduled_tasks (enabled, next_run_at);
CREATE INDEX idx_automation_scheduled_tasks_runtime_running ON automation_scheduled_tasks (running_run_id);
CREATE INDEX idx_automation_scheduled_tasks_expires_at ON automation_scheduled_tasks (enabled, expires_at);
CREATE INDEX idx_automation_task_runs_job ON automation_task_runs (job_id);
CREATE INDEX idx_automation_task_runs_status ON automation_task_runs (status);
CREATE INDEX idx_automation_task_runs_job_created ON automation_task_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX idx_automation_task_runs_owner_job_created ON automation_task_runs (owner_user_id, job_id, created_at DESC, run_id DESC);

UPDATE automation_task_runs SET trigger_kind = 'scheduled' WHERE trigger_kind = 'cron';

-- +goose Down
UPDATE automation_task_runs SET trigger_kind = 'cron' WHERE trigger_kind = 'scheduled';

DROP INDEX IF EXISTS idx_automation_scheduled_tasks_agent;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_created;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_agent_created;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_enabled_agent;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_owner_created;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_owner_agent_created;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_owner_enabled_agent;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_runtime_due;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_runtime_running;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_expires_at;
DROP INDEX IF EXISTS idx_automation_task_runs_job;
DROP INDEX IF EXISTS idx_automation_task_runs_status;
DROP INDEX IF EXISTS idx_automation_task_runs_job_created;
DROP INDEX IF EXISTS idx_automation_task_runs_owner_job_created;

ALTER TABLE automation_task_runs RENAME TO automation_cron_runs;
ALTER TABLE automation_scheduled_tasks RENAME TO automation_cron_jobs;

CREATE INDEX idx_automation_cron_jobs_agent ON automation_cron_jobs (agent_id);
CREATE INDEX idx_automation_cron_jobs_created ON automation_cron_jobs (created_at DESC, job_id DESC);
CREATE INDEX idx_automation_cron_jobs_agent_created ON automation_cron_jobs (agent_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_cron_jobs_enabled_agent ON automation_cron_jobs (enabled, agent_id);
CREATE INDEX idx_automation_cron_jobs_owner_created ON automation_cron_jobs (owner_user_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_cron_jobs_owner_agent_created ON automation_cron_jobs (owner_user_id, agent_id, created_at DESC, job_id DESC);
CREATE INDEX idx_automation_cron_jobs_owner_enabled_agent ON automation_cron_jobs (owner_user_id, enabled, agent_id);
CREATE INDEX idx_automation_cron_jobs_runtime_due ON automation_cron_jobs (enabled, next_run_at);
CREATE INDEX idx_automation_cron_jobs_runtime_running ON automation_cron_jobs (running_run_id);
CREATE INDEX idx_automation_cron_jobs_expires_at ON automation_cron_jobs (enabled, expires_at);
CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);
CREATE INDEX idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX idx_automation_cron_runs_owner_job_created ON automation_cron_runs (owner_user_id, job_id, created_at DESC, run_id DESC);
