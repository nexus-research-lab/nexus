-- +goose Up
CREATE TABLE sandbox_scratch_recoveries (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 lease_id TEXT NOT NULL,
 source_generation BIGINT NOT NULL CHECK (source_generation > 0),
 source_launch_id TEXT NOT NULL,
 scratch_json TEXT NOT NULL,
 phase TEXT NOT NULL CHECK (phase IN ('prepared','quarantined','deleting','complete')),
 PRIMARY KEY (owner_user_id,session_key,lease_id)
);
CREATE INDEX idx_sandbox_scratch_pending ON sandbox_scratch_recoveries(owner_user_id,session_key) WHERE phase <> 'complete';
-- +goose Down
CREATE TABLE sandbox_scratch_rollback_guard (record_count BIGINT CHECK (record_count = 0));
INSERT INTO sandbox_scratch_rollback_guard SELECT COUNT(*) FROM sandbox_scratch_recoveries;
DROP TABLE sandbox_scratch_rollback_guard;
DROP TABLE sandbox_scratch_recoveries;
