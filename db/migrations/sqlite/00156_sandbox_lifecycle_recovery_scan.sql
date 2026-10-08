-- +goose Up
ALTER TABLE sandbox_process_launches ADD COLUMN resource_phase TEXT NOT NULL DEFAULT 'pending' CHECK (resource_phase IN ('pending','complete'));
CREATE INDEX idx_sandbox_resource_recovery ON sandbox_process_launches(launch_id) WHERE resource_phase='pending';
CREATE INDEX idx_sandbox_policy_recovery ON sandbox_policy_receipts(process_launch_id) WHERE process_launch_id<>'' AND phase IN ('confirmed','retiring','unknown');
-- +goose Down
-- Do not discard recovery scan progress while original launch records exist.
CREATE TABLE sandbox_lifecycle_rollback_guard (record_count BIGINT CHECK (record_count = 0));
INSERT INTO sandbox_lifecycle_rollback_guard SELECT COUNT(*) FROM sandbox_process_launches;
DROP TABLE sandbox_lifecycle_rollback_guard;
DROP INDEX idx_sandbox_policy_recovery;
DROP INDEX idx_sandbox_resource_recovery;
ALTER TABLE sandbox_process_launches DROP COLUMN resource_phase;
