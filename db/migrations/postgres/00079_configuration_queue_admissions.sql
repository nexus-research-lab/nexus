-- +goose Up
-- Keep the trust decision for queued conversational configuration input outside
-- Agent-writable workspace JSONL. A claim is single-use and payload-bound.
CREATE TABLE configuration_queue_admissions (
    owner_user_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    queue_item_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    session_key TEXT NOT NULL,
    room_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    source_message_id TEXT NOT NULL DEFAULT '',
    principal_user_id TEXT NOT NULL DEFAULT '',
    target_agent_ids_json TEXT NOT NULL DEFAULT '[]',
    payload_digest TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    claim_token TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    claimed_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, scope, queue_item_id, agent_id),
    CONSTRAINT ck_configuration_queue_admissions_scope
        CHECK (scope IN ('dm', 'room')),
    CONSTRAINT ck_configuration_queue_admissions_status
        CHECK (status IN ('pending', 'claimed', 'consumed', 'revoked'))
);

CREATE INDEX idx_configuration_queue_admissions_status
    ON configuration_queue_admissions(status, updated_at);

-- +goose Down
DROP INDEX IF EXISTS idx_configuration_queue_admissions_status;
DROP TABLE IF EXISTS configuration_queue_admissions;
