-- +goose Up
ALTER TABLE automation_task_runs ADD COLUMN IF NOT EXISTS delivery_attempt_id VARCHAR(64);
ALTER TABLE automation_task_runs ADD COLUMN IF NOT EXISTS delivery_attempt_started_at TIMESTAMPTZ;
ALTER TABLE automation_scheduled_tasks ADD COLUMN IF NOT EXISTS last_completed_run_id VARCHAR(64);
-- Bind legacy task delivery summaries to the latest actual terminal execution.
-- Skipped overlap/misfire rows do not own last_delivery_status, and active or
-- unfinished rows can never become the delivery summary authority.
UPDATE automation_scheduled_tasks
SET last_completed_run_id = (
    SELECT run.run_id
    FROM automation_task_runs AS run
    WHERE run.owner_user_id = automation_scheduled_tasks.owner_user_id
      AND run.job_id = automation_scheduled_tasks.job_id
      AND run.status IN ('succeeded', 'failed', 'cancelled')
      AND run.finished_at IS NOT NULL
    ORDER BY run.finished_at DESC, run.run_id DESC
    LIMIT 1
)
WHERE last_completed_run_id IS NULL
  AND EXISTS (
      SELECT 1
      FROM automation_task_runs AS run
      WHERE run.owner_user_id = automation_scheduled_tasks.owner_user_id
        AND run.job_id = automation_scheduled_tasks.job_id
        AND run.status IN ('succeeded', 'failed', 'cancelled')
        AND run.finished_at IS NOT NULL
  );
CREATE INDEX IF NOT EXISTS idx_automation_task_runs_delivery_due
    ON automation_task_runs (delivery_status, delivery_next_attempt_at, updated_at, run_id)
    WHERE delivery_dead_letter_at IS NULL
      AND delivery_status IN ('pending', 'failed');

-- +goose Down
DROP INDEX IF EXISTS idx_automation_task_runs_delivery_due;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS last_completed_run_id;
ALTER TABLE automation_task_runs DROP COLUMN IF EXISTS delivery_attempt_started_at;
ALTER TABLE automation_task_runs DROP COLUMN IF EXISTS delivery_attempt_id;
