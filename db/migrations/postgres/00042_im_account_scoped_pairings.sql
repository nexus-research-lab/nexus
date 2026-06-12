-- +goose Up
ALTER TABLE im_pairings ADD COLUMN IF NOT EXISTS account_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE im_pairings DROP CONSTRAINT IF EXISTS uq_im_pairings_target;
ALTER TABLE im_pairings
    ADD CONSTRAINT uq_im_pairings_target UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id);

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
ALTER TABLE im_ingress_messages ADD COLUMN IF NOT EXISTS account_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE im_ingress_messages DROP CONSTRAINT IF EXISTS im_ingress_messages_pkey;
ALTER TABLE im_ingress_messages
    ADD CONSTRAINT im_ingress_messages_pkey PRIMARY KEY (owner_user_id, channel_type, account_id, req_id);

-- +goose Down
ALTER TABLE im_ingress_messages DROP CONSTRAINT IF EXISTS im_ingress_messages_pkey;
ALTER TABLE im_ingress_messages
    ADD CONSTRAINT im_ingress_messages_pkey PRIMARY KEY (owner_user_id, channel_type, req_id);
ALTER TABLE im_ingress_messages DROP COLUMN IF EXISTS account_id;

ALTER TABLE im_pairings DROP CONSTRAINT IF EXISTS uq_im_pairings_target;
ALTER TABLE im_pairings
    ADD CONSTRAINT uq_im_pairings_target UNIQUE (owner_user_id, channel_type, chat_type, external_ref, thread_id);
ALTER TABLE im_pairings DROP COLUMN IF EXISTS account_id;
