-- +goose Up
CREATE TABLE windows_sandbox_scopes (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL DEFAULT 0,
 launch_order INTEGER NOT NULL DEFAULT 0,
 active_launch_id TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(owner_user_id,session_key)
);
CREATE TABLE windows_sandbox_launches (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL CHECK (generation > 0),
 launch_id TEXT NOT NULL UNIQUE,
 launch_order INTEGER NOT NULL CHECK (launch_order IN (1,2)),
 phase TEXT NOT NULL CHECK (phase IN ('reserved','prepared','started','unknown','cleaned')),
 intent_json TEXT NOT NULL,
 prepared_json TEXT NOT NULL DEFAULT '',
 outcome_json TEXT NOT NULL DEFAULT '',
 PRIMARY KEY (owner_user_id,session_key,generation,launch_order)
);
CREATE UNIQUE INDEX idx_windows_sandbox_pending ON windows_sandbox_launches(owner_user_id,session_key) WHERE phase <> 'cleaned';
CREATE INDEX idx_windows_sandbox_latest ON windows_sandbox_launches(owner_user_id,session_key,generation DESC,launch_order DESC);
-- +goose Down
DROP INDEX idx_windows_sandbox_latest;
DROP INDEX idx_windows_sandbox_pending;
DROP TABLE windows_sandbox_launches;
DROP TABLE windows_sandbox_scopes;
