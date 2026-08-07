-- +goose Up

CREATE TABLE deletion_jobs (
    job_id VARCHAR(80) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    target_id VARCHAR(512) NOT NULL,
    payload_json JSON NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_deletion_jobs_kind CHECK (
        kind IN ('session', 'room', 'conversation', 'room_member', 'agent', 'scheduled_task', 'skill')
    ),
    CONSTRAINT uq_deletion_jobs_target UNIQUE (owner_user_id, kind, target_id)
);

CREATE INDEX idx_deletion_jobs_pending
    ON deletion_jobs (updated_at, job_id);

CREATE TABLE session_transcript_refs (
    room_session_id VARCHAR(64) NOT NULL,
    sdk_session_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (room_session_id, sdk_session_id),
    FOREIGN KEY(room_session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_session_transcript_refs_sdk
    ON session_transcript_refs (sdk_session_id, room_session_id);

INSERT INTO session_transcript_refs (room_session_id, sdk_session_id)
SELECT id, TRIM(sdk_session_id)
FROM sessions
WHERE sdk_session_id IS NOT NULL AND TRIM(sdk_session_id) <> ''
ON CONFLICT DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS session_transcript_refs;
DROP TABLE IF EXISTS deletion_jobs;
