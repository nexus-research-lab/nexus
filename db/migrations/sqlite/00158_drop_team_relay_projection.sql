-- +goose Up
-- Relay 是在线消息的唯一权威；本地投影没有读取方，且其连续性失败会让读取接口持续 503。
DROP TABLE team_relay_messages;
DROP TABLE team_relay_owner_cursors;
DROP TABLE team_relay_conversations;

-- +goose Down
CREATE TABLE team_relay_conversations (
    deployment_id TEXT NOT NULL,
    team_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    stream_id TEXT NOT NULL UNIQUE,
    stream_epoch TEXT NOT NULL,
    next_room_seq INTEGER NOT NULL DEFAULT 1 CHECK (next_room_seq > 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deployment_id, conversation_id)
);

CREATE TABLE team_relay_owner_cursors (
    owner_user_id TEXT NOT NULL,
    deployment_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    relay_seq INTEGER NOT NULL DEFAULT 0 CHECK (relay_seq >= 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, deployment_id, conversation_id),
    FOREIGN KEY (owner_user_id)
        REFERENCES owner_profiles (owner_user_id) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id, conversation_id)
        REFERENCES team_relay_conversations (deployment_id, conversation_id) ON DELETE CASCADE
);

CREATE TABLE team_relay_messages (
    deployment_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    message_seq INTEGER NOT NULL CHECK (message_seq > 0),
    room_seq INTEGER NOT NULL CHECK (room_seq > 0),
    author_type TEXT NOT NULL,
    author_user_id TEXT NOT NULL,
    author_username TEXT NOT NULL,
    author_display_name TEXT NOT NULL,
    client_message_id TEXT NOT NULL,
    content_json TEXT NOT NULL CHECK (json_valid(content_json)),
    created_at DATETIME NOT NULL,
    projected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deployment_id, conversation_id, message_id),
    UNIQUE (deployment_id, conversation_id, message_seq),
    UNIQUE (deployment_id, conversation_id, room_seq),
    FOREIGN KEY (deployment_id, conversation_id)
        REFERENCES team_relay_conversations (deployment_id, conversation_id) ON DELETE CASCADE
);

ALTER TABLE team_relay_messages ADD COLUMN author_agent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN delivery_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN output_kind TEXT NOT NULL DEFAULT '';
