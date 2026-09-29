-- +goose Up
ALTER TABLE sandbox_policy_receipts ADD COLUMN process_generation BIGINT NOT NULL DEFAULT 0 CHECK (process_generation >= 0 AND process_generation <= generation);
ALTER TABLE sandbox_policy_receipts ADD COLUMN process_launch_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_sandbox_policy_process ON sandbox_policy_receipts(owner_user_id, session_key, process_launch_id) WHERE process_launch_id <> '';
-- +goose Down
-- Refuse to discard the only durable mapping between warm policy and process.
CREATE TABLE sandbox_policy_binding_rollback_guard (bound_count BIGINT CHECK (bound_count = 0));
INSERT INTO sandbox_policy_binding_rollback_guard SELECT COUNT(*) FROM sandbox_policy_receipts WHERE process_launch_id <> '';
DROP TABLE sandbox_policy_binding_rollback_guard;
DROP INDEX idx_sandbox_policy_process;
ALTER TABLE sandbox_policy_receipts DROP COLUMN process_launch_id;
ALTER TABLE sandbox_policy_receipts DROP COLUMN process_generation;
