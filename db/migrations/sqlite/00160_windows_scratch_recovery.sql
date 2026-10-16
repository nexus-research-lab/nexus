-- +goose Up
CREATE TABLE windows_scratch_recoveries (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 lease_id TEXT NOT NULL,
 launch_id TEXT NOT NULL,
 phase TEXT NOT NULL CHECK(phase IN ('prepared','quarantined','deleting','removed','complete')),
 record_json TEXT NOT NULL,
 PRIMARY KEY(owner_user_id,session_key,lease_id)
);
-- +goose Down
CREATE TABLE windows_scratch_rollback_guard(pending BIGINT CHECK(pending=0));
INSERT INTO windows_scratch_rollback_guard SELECT COUNT(*) FROM windows_scratch_recoveries;
DROP TABLE windows_scratch_rollback_guard;
DROP TABLE windows_scratch_recoveries;
