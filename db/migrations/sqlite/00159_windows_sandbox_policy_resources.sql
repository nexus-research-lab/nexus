-- +goose Up
ALTER TABLE sandbox_policy_receipts ADD COLUMN windows_process_json TEXT NOT NULL DEFAULT '';
CREATE TABLE windows_sandbox_resources (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 lease_id TEXT NOT NULL,
 launch_id TEXT NOT NULL,
 scratch_root TEXT NOT NULL,
 phase TEXT NOT NULL CHECK(phase IN ('pending','complete')),
 PRIMARY KEY(owner_user_id,session_key,lease_id)
);
-- +goose Down
CREATE TABLE windows_sandbox_rollback_guard(bound_count BIGINT CHECK(bound_count=0));
INSERT INTO windows_sandbox_rollback_guard SELECT COUNT(*) FROM sandbox_policy_receipts WHERE windows_process_json<>'';
INSERT INTO windows_sandbox_rollback_guard SELECT COUNT(*) FROM windows_sandbox_resources;
DROP TABLE windows_sandbox_rollback_guard;
DROP TABLE windows_sandbox_resources;
ALTER TABLE sandbox_policy_receipts DROP COLUMN windows_process_json;
