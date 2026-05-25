-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS assistant_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS result_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS artifact_path VARCHAR(512);

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS artifact_path;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS result_text;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS assistant_text;
