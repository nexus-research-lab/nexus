-- +goose Up
CREATE TABLE sandbox_process_launches (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL CHECK (generation > 0),
 launch_id TEXT NOT NULL UNIQUE,
 phase TEXT NOT NULL CHECK (phase IN ('prepared', 'registered', 'released', 'aborted', 'reaped')),
 intent_json TEXT NOT NULL,
 registration_json TEXT NOT NULL DEFAULT '',
 evidence_json TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (owner_user_id, session_key, generation)
);
CREATE UNIQUE INDEX idx_sandbox_process_active
 ON sandbox_process_launches(owner_user_id, session_key)
 WHERE phase IN ('prepared', 'registered', 'released');
CREATE INDEX idx_sandbox_process_latest
 ON sandbox_process_launches(owner_user_id, session_key, generation DESC);
-- +goose Down
DROP INDEX idx_sandbox_process_latest;
DROP INDEX idx_sandbox_process_active;
DROP TABLE sandbox_process_launches;
