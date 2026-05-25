-- +goose Up
CREATE TABLE IF NOT EXISTS im_ingress_messages (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    PRIMARY KEY (owner_user_id, channel_type, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_im_ingress_messages_updated ON im_ingress_messages (updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_im_ingress_messages_updated;
DROP TABLE IF EXISTS im_ingress_messages;
