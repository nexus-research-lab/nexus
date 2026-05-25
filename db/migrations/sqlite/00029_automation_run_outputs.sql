-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN assistant_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN result_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN artifact_path VARCHAR(512);

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN artifact_path;
ALTER TABLE automation_cron_runs DROP COLUMN result_text;
ALTER TABLE automation_cron_runs DROP COLUMN assistant_text;
