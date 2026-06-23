-- +goose Up
DROP INDEX IF EXISTS idx_im_pairings_agent;
DROP INDEX IF EXISTS idx_im_pairings_owner_channel_status;
DROP INDEX IF EXISTS idx_im_pairings_owner_status;

CREATE TABLE im_pairings_new (
    pairing_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    account_id VARCHAR(255) NOT NULL DEFAULT '',
    chat_type VARCHAR(32) NOT NULL,
    external_ref VARCHAR(255) NOT NULL,
    thread_id VARCHAR(255) NOT NULL DEFAULT '',
    external_name VARCHAR(255),
    agent_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    last_message_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_im_pairings_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_pairings_chat_type CHECK (chat_type IN ('dm', 'group')),
    CONSTRAINT ck_im_pairings_status CHECK (status IN ('pending', 'active', 'disabled', 'rejected')),
    CONSTRAINT ck_im_pairings_source CHECK (source IN ('manual', 'ingress', 'wechat_qr')),
    CONSTRAINT uq_im_pairings_target UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT INTO im_pairings_new (
    pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at, created_at, updated_at
)
SELECT
    pairing_id, owner_user_id, channel_type, '', chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings;

DROP TABLE im_pairings;
ALTER TABLE im_pairings_new RENAME TO im_pairings;

CREATE INDEX idx_im_pairings_owner_status ON im_pairings (owner_user_id, status, updated_at DESC);
CREATE INDEX idx_im_pairings_owner_channel_status ON im_pairings (owner_user_id, channel_type, status);
CREATE INDEX idx_im_pairings_agent ON im_pairings (agent_id, status);

CREATE TABLE IF NOT EXISTS im_ingress_messages (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    PRIMARY KEY (owner_user_id, channel_type, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);
DROP INDEX IF EXISTS idx_im_ingress_messages_updated;

CREATE TABLE im_ingress_messages_new (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    account_id VARCHAR(255) NOT NULL DEFAULT '',
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    PRIMARY KEY (owner_user_id, channel_type, account_id, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);

INSERT INTO im_ingress_messages_new (
    owner_user_id, channel_type, account_id, req_id, agent_id, session_key, round_id,
    status, error_message, created_at, updated_at, completed_at
)
SELECT
    owner_user_id, channel_type, '', req_id, agent_id, session_key, round_id,
    status, error_message, created_at, updated_at, completed_at
FROM im_ingress_messages;

DROP TABLE im_ingress_messages;
ALTER TABLE im_ingress_messages_new RENAME TO im_ingress_messages;

CREATE INDEX idx_im_ingress_messages_updated ON im_ingress_messages (updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_im_ingress_messages_updated;

CREATE TABLE im_ingress_messages_old (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    PRIMARY KEY (owner_user_id, channel_type, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);

INSERT OR IGNORE INTO im_ingress_messages_old (
    owner_user_id, channel_type, req_id, agent_id, session_key, round_id,
    status, error_message, created_at, updated_at, completed_at
)
SELECT
    owner_user_id, channel_type, req_id, agent_id, session_key, round_id,
    status, error_message, created_at, updated_at, completed_at
FROM im_ingress_messages
ORDER BY updated_at DESC;

DROP TABLE im_ingress_messages;
ALTER TABLE im_ingress_messages_old RENAME TO im_ingress_messages;

CREATE INDEX idx_im_ingress_messages_updated ON im_ingress_messages (updated_at DESC);

DROP INDEX IF EXISTS idx_im_pairings_agent;
DROP INDEX IF EXISTS idx_im_pairings_owner_channel_status;
DROP INDEX IF EXISTS idx_im_pairings_owner_status;

CREATE TABLE im_pairings_old (
    pairing_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    chat_type VARCHAR(32) NOT NULL,
    external_ref VARCHAR(255) NOT NULL,
    thread_id VARCHAR(255) NOT NULL DEFAULT '',
    external_name VARCHAR(255),
    agent_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    last_message_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_im_pairings_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_pairings_chat_type CHECK (chat_type IN ('dm', 'group')),
    CONSTRAINT ck_im_pairings_status CHECK (status IN ('pending', 'active', 'disabled', 'rejected')),
    CONSTRAINT ck_im_pairings_source CHECK (source IN ('manual', 'ingress', 'wechat_qr')),
    CONSTRAINT uq_im_pairings_target UNIQUE (owner_user_id, channel_type, chat_type, external_ref, thread_id),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO im_pairings_old (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at, created_at, updated_at
)
SELECT
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id, external_name,
    agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
ORDER BY updated_at DESC;

DROP TABLE im_pairings;
ALTER TABLE im_pairings_old RENAME TO im_pairings;

CREATE INDEX idx_im_pairings_owner_status ON im_pairings (owner_user_id, status, updated_at DESC);
CREATE INDEX idx_im_pairings_owner_channel_status ON im_pairings (owner_user_id, channel_type, status);
CREATE INDEX idx_im_pairings_agent ON im_pairings (agent_id, status);
