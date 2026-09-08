-- +goose Up
ALTER TABLE team_relay_messages RENAME TO team_relay_messages_owner_v134;
ALTER TABLE team_relay_conversations RENAME TO team_relay_conversations_owner_v134;

CREATE TABLE team_relay_conversations_shared_v135 (
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

CREATE TABLE team_relay_owner_cursors_shared_v135 (
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
        REFERENCES team_relay_conversations_shared_v135 (deployment_id, conversation_id) ON DELETE CASCADE
);

CREATE TABLE team_relay_messages_shared_v135 (
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
        REFERENCES team_relay_conversations_shared_v135 (deployment_id, conversation_id) ON DELETE CASCADE
);

INSERT INTO team_relay_conversations_shared_v135 (
    deployment_id, team_id, room_id, conversation_id, stream_id, stream_epoch,
    next_room_seq, created_at, updated_at
)
SELECT
    deployment_id, team_id, room_id, conversation_id, stream_id, stream_epoch,
    1, created_at, updated_at
FROM (
    SELECT
        legacy.*,
        ROW_NUMBER() OVER (
            PARTITION BY deployment_id, conversation_id
            ORDER BY updated_at DESC, owner_user_id
        ) AS generation_rank
    FROM team_relay_conversations_owner_v134 AS legacy
) AS ranked
WHERE generation_rank = 1;

INSERT INTO team_relay_owner_cursors_shared_v135 (
    owner_user_id, deployment_id, conversation_id, relay_seq, created_at, updated_at
)
SELECT
    legacy.owner_user_id,
    legacy.deployment_id,
    legacy.conversation_id,
    CASE WHEN legacy.stream_epoch = shared.stream_epoch THEN legacy.relay_seq ELSE 0 END,
    legacy.created_at,
    legacy.updated_at
FROM team_relay_conversations_owner_v134 AS legacy
JOIN team_relay_conversations_shared_v135 AS shared
  ON shared.deployment_id = legacy.deployment_id
 AND shared.conversation_id = legacy.conversation_id;

WITH ranked AS (
    SELECT
        conversation.deployment_id,
        message.conversation_id,
        message.message_id,
        message.message_seq,
        message.author_type,
        message.author_user_id,
        message.author_username,
        message.author_display_name,
        message.client_message_id,
        message.content_json,
        message.created_at,
        message.projected_at,
        ROW_NUMBER() OVER (
            PARTITION BY conversation.deployment_id, message.conversation_id, message.message_seq
            ORDER BY message.projected_at, message.owner_user_id, message.message_id
        ) AS message_rank
    FROM team_relay_messages_owner_v134 AS message
    JOIN team_relay_conversations_owner_v134 AS conversation
      ON conversation.owner_user_id = message.owner_user_id
     AND conversation.conversation_id = message.conversation_id
    JOIN team_relay_conversations_shared_v135 AS shared
      ON shared.deployment_id = conversation.deployment_id
     AND shared.conversation_id = conversation.conversation_id
     AND shared.stream_epoch = conversation.stream_epoch
), deduplicated AS (
    SELECT * FROM ranked WHERE message_rank = 1
)
INSERT INTO team_relay_messages_shared_v135 (
    deployment_id, conversation_id, message_id, message_seq, room_seq,
    author_type, author_user_id, author_username, author_display_name,
    client_message_id, content_json, created_at, projected_at
)
SELECT
    deployment_id,
    conversation_id,
    message_id,
    message_seq,
    ROW_NUMBER() OVER (
        PARTITION BY deployment_id, conversation_id
        ORDER BY message_seq, message_id
    ),
    author_type,
    author_user_id,
    author_username,
    author_display_name,
    client_message_id,
    content_json,
    created_at,
    projected_at
FROM deduplicated;

UPDATE team_relay_conversations_shared_v135 AS conversation
SET next_room_seq = message.next_room_seq
FROM (
    SELECT deployment_id, conversation_id, MAX(room_seq) + 1 AS next_room_seq
    FROM team_relay_messages_shared_v135
    GROUP BY deployment_id, conversation_id
) AS message
WHERE message.deployment_id = conversation.deployment_id
  AND message.conversation_id = conversation.conversation_id;

DROP TABLE team_relay_messages_owner_v134;
DROP TABLE team_relay_conversations_owner_v134;

ALTER TABLE team_relay_conversations_shared_v135 RENAME TO team_relay_conversations;
ALTER TABLE team_relay_owner_cursors_shared_v135 RENAME TO team_relay_owner_cursors;
ALTER TABLE team_relay_messages_shared_v135 RENAME TO team_relay_messages;

-- +goose Down
ALTER TABLE team_relay_messages RENAME TO team_relay_messages_shared_v135;
ALTER TABLE team_relay_owner_cursors RENAME TO team_relay_owner_cursors_shared_v135;
ALTER TABLE team_relay_conversations RENAME TO team_relay_conversations_shared_v135;

CREATE TABLE team_relay_conversations_owner_v134 (
    owner_user_id VARCHAR(128) NOT NULL,
    deployment_id VARCHAR(128) NOT NULL,
    team_id VARCHAR(128) NOT NULL,
    room_id VARCHAR(128) NOT NULL,
    conversation_id VARCHAR(128) NOT NULL,
    stream_id VARCHAR(128) NOT NULL,
    stream_epoch VARCHAR(128) NOT NULL,
    relay_seq BIGINT NOT NULL DEFAULT 0 CHECK (relay_seq >= 0),
    next_room_seq BIGINT NOT NULL DEFAULT 1 CHECK (next_room_seq > 0),
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, conversation_id),
    UNIQUE (owner_user_id, stream_id),
    FOREIGN KEY (owner_user_id)
        REFERENCES owner_profiles (owner_user_id) ON DELETE CASCADE
);

CREATE TABLE team_relay_messages_owner_v134 (
    owner_user_id VARCHAR(128) NOT NULL,
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
    PRIMARY KEY (owner_user_id, message_id),
    UNIQUE (owner_user_id, conversation_id, message_seq),
    UNIQUE (owner_user_id, conversation_id, room_seq),
    FOREIGN KEY (owner_user_id, conversation_id)
        REFERENCES team_relay_conversations_owner_v134 (owner_user_id, conversation_id) ON DELETE CASCADE
);

INSERT INTO team_relay_conversations_owner_v134 (
    owner_user_id, deployment_id, team_id, room_id, conversation_id,
    stream_id, stream_epoch, relay_seq, next_room_seq, created_at, updated_at
)
SELECT
    cursor.owner_user_id,
    conversation.deployment_id,
    conversation.team_id,
    conversation.room_id,
    conversation.conversation_id,
    conversation.stream_id,
    conversation.stream_epoch,
    cursor.relay_seq,
    conversation.next_room_seq,
    conversation.created_at,
    cursor.updated_at
FROM team_relay_owner_cursors_shared_v135 AS cursor
JOIN team_relay_conversations_shared_v135 AS conversation
  ON conversation.deployment_id = cursor.deployment_id
 AND conversation.conversation_id = cursor.conversation_id;

INSERT INTO team_relay_messages_owner_v134 (
    owner_user_id, conversation_id, message_id, message_seq, room_seq,
    author_type, author_user_id, author_username, author_display_name,
    client_message_id, content_json, created_at, projected_at
)
SELECT
    cursor.owner_user_id,
    message.conversation_id,
    message.message_id,
    message.message_seq,
    message.room_seq,
    message.author_type,
    message.author_user_id,
    message.author_username,
    message.author_display_name,
    message.client_message_id,
    message.content_json,
    message.created_at,
    message.projected_at
FROM team_relay_owner_cursors_shared_v135 AS cursor
JOIN team_relay_messages_shared_v135 AS message
  ON message.deployment_id = cursor.deployment_id
 AND message.conversation_id = cursor.conversation_id;

DROP TABLE team_relay_messages_shared_v135;
DROP TABLE team_relay_owner_cursors_shared_v135;
DROP TABLE team_relay_conversations_shared_v135;

ALTER TABLE team_relay_conversations_owner_v134 RENAME TO team_relay_conversations;
ALTER TABLE team_relay_messages_owner_v134 RENAME TO team_relay_messages;
