-- +goose Up
-- Relay 是在线消息的唯一权威；本地投影没有读取方，且其连续性失败会让读取接口持续 503。
DROP TABLE team_relay_messages;
DROP TABLE team_relay_owner_cursors;
DROP TABLE team_relay_conversations;

-- +goose Down
CREATE TABLE team_relay_conversations (
    deployment_id VARCHAR(128) NOT NULL,
    team_id VARCHAR(128) NOT NULL,
    room_id VARCHAR(128) NOT NULL,
    conversation_id VARCHAR(128) NOT NULL,
    stream_id VARCHAR(128) NOT NULL UNIQUE,
    stream_epoch VARCHAR(128) NOT NULL,
    next_room_seq BIGINT NOT NULL DEFAULT 1 CHECK (next_room_seq > 0),
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deployment_id, conversation_id)
);

CREATE TABLE team_relay_owner_cursors (
    owner_user_id VARCHAR(128) NOT NULL,
    deployment_id VARCHAR(128) NOT NULL,
    conversation_id VARCHAR(128) NOT NULL,
    relay_seq BIGINT NOT NULL DEFAULT 0 CHECK (relay_seq >= 0),
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, deployment_id, conversation_id),
    FOREIGN KEY (owner_user_id)
        REFERENCES owner_profiles (owner_user_id) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id, conversation_id)
        REFERENCES team_relay_conversations (deployment_id, conversation_id) ON DELETE CASCADE
);

CREATE TABLE team_relay_messages (
    deployment_id VARCHAR(128) NOT NULL,
    conversation_id VARCHAR(128) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    message_seq BIGINT NOT NULL CHECK (message_seq > 0),
    room_seq BIGINT NOT NULL CHECK (room_seq > 0),
    author_type VARCHAR(32) NOT NULL,
    author_user_id VARCHAR(128) NOT NULL,
    author_username VARCHAR(128) NOT NULL,
    author_display_name VARCHAR(256) NOT NULL,
    client_message_id VARCHAR(128) NOT NULL,
    content_json JSONB NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    projected_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deployment_id, conversation_id, message_id),
    UNIQUE (deployment_id, conversation_id, message_seq),
    UNIQUE (deployment_id, conversation_id, room_seq),
    FOREIGN KEY (deployment_id, conversation_id)
        REFERENCES team_relay_conversations (deployment_id, conversation_id) ON DELETE CASCADE
);

ALTER TABLE team_relay_messages ADD COLUMN author_agent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN delivery_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN output_kind TEXT NOT NULL DEFAULT '';
