-- +goose Up
CREATE INDEX idx_sandbox_process_pending_launch
 ON sandbox_process_launches(launch_id)
 WHERE phase IN ('prepared', 'registered', 'released');
-- +goose Down
DROP INDEX idx_sandbox_process_pending_launch;
