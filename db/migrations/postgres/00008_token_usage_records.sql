-- +goose Up
CREATE TABLE token_usage_records (
    owner_user_id VARCHAR(64) NOT NULL,
    usage_key VARCHAR(512) NOT NULL,
    source VARCHAR(64) NOT NULL,
    session_key VARCHAR(256) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    round_id VARCHAR(128) NOT NULL DEFAULT '',
    agent_id VARCHAR(128) NOT NULL DEFAULT '',
    room_id VARCHAR(128) NOT NULL DEFAULT '',
    conversation_id VARCHAR(128) NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_input_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, usage_key)
);
CREATE INDEX idx_token_usage_records_owner_time ON token_usage_records (owner_user_id, occurred_at);
CREATE INDEX idx_token_usage_records_session ON token_usage_records (session_key);

-- +goose Down
DROP INDEX IF EXISTS idx_token_usage_records_session;
DROP INDEX IF EXISTS idx_token_usage_records_owner_time;
DROP TABLE IF EXISTS token_usage_records;
