-- +goose Up
CREATE TABLE sandbox_process_launches_v2 (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL CHECK (generation > 0),
 launch_order INTEGER NOT NULL DEFAULT 4 CHECK (launch_order BETWEEN 1 AND 4),
 launch_id TEXT NOT NULL UNIQUE,
 phase TEXT NOT NULL CHECK (phase IN ('prepared', 'registered', 'released', 'aborted', 'reaped')),
 intent_json TEXT NOT NULL,
 registration_json TEXT NOT NULL DEFAULT '',
 evidence_json TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 PRIMARY KEY (owner_user_id, session_key, generation, launch_order)
);
INSERT INTO sandbox_process_launches_v2 (owner_user_id,session_key,generation,launch_id,phase,intent_json,registration_json,evidence_json,created_at,updated_at) SELECT owner_user_id,session_key,generation,launch_id,phase,intent_json,registration_json,evidence_json,created_at,updated_at FROM sandbox_process_launches;
DROP TABLE sandbox_process_launches;
ALTER TABLE sandbox_process_launches_v2 RENAME TO sandbox_process_launches;
CREATE UNIQUE INDEX idx_sandbox_process_active ON sandbox_process_launches(owner_user_id,session_key) WHERE phase IN ('prepared','registered','released');
CREATE INDEX idx_sandbox_process_latest ON sandbox_process_launches(owner_user_id,session_key,generation DESC,launch_order DESC);
-- +goose Down
-- Refuse rollback when a generation contains multiple launches; never discard evidence.
CREATE TABLE sandbox_process_launches_v1 (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL CHECK (generation > 0),
 launch_id TEXT NOT NULL UNIQUE,
 phase TEXT NOT NULL CHECK (phase IN ('prepared', 'registered', 'released', 'aborted', 'reaped')),
 intent_json TEXT NOT NULL,
 registration_json TEXT NOT NULL DEFAULT '',
 evidence_json TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 PRIMARY KEY (owner_user_id, session_key, generation)
);
INSERT INTO sandbox_process_launches_v1 (owner_user_id,session_key,generation,launch_id,phase,intent_json,registration_json,evidence_json,created_at,updated_at) SELECT owner_user_id,session_key,generation,launch_id,phase,intent_json,registration_json,evidence_json,created_at,updated_at FROM sandbox_process_launches;
DROP TABLE sandbox_process_launches;
ALTER TABLE sandbox_process_launches_v1 RENAME TO sandbox_process_launches;
CREATE UNIQUE INDEX idx_sandbox_process_active ON sandbox_process_launches(owner_user_id,session_key) WHERE phase IN ('prepared','registered','released');
CREATE INDEX idx_sandbox_process_latest ON sandbox_process_launches(owner_user_id,session_key,generation DESC);
