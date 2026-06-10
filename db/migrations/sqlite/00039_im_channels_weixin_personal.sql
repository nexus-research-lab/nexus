-- +goose Up
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_im_pairings_agent;
DROP INDEX IF EXISTS idx_im_pairings_owner_channel_status;
DROP INDEX IF EXISTS idx_im_pairings_owner_status;
DROP INDEX IF EXISTS idx_im_channel_configs_owner_status;

CREATE TABLE im_channel_configs_new (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'configured',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type),
    CONSTRAINT ck_im_channel_configs_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_channel_configs_status CHECK (status IN ('configured', 'connected', 'pending', 'error', 'disabled')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE RESTRICT
);

INSERT INTO im_channel_configs_new (
    owner_user_id, channel_type, agent_id, status, config_json,
    credentials_encrypted, last_error, created_at, updated_at
)
SELECT
    owner_user_id,
    CASE WHEN channel_type = 'openclaw-weixin' THEN 'weixin-personal' ELSE channel_type END,
    agent_id, status, config_json,
    credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs;

DROP TABLE im_channel_configs;
ALTER TABLE im_channel_configs_new RENAME TO im_channel_configs;

CREATE TABLE im_pairings_new (
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

INSERT INTO im_pairings_new (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id,
    external_name, agent_id, status, source, last_message_at, created_at, updated_at
)
SELECT
    pairing_id,
    owner_user_id,
    CASE WHEN channel_type = 'openclaw-weixin' THEN 'weixin-personal' ELSE channel_type END,
    chat_type, external_ref, thread_id,
    external_name, agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings;

DROP TABLE im_pairings;
ALTER TABLE im_pairings_new RENAME TO im_pairings;

CREATE INDEX idx_im_channel_configs_owner_status ON im_channel_configs (owner_user_id, status, channel_type);
CREATE INDEX idx_im_pairings_owner_status ON im_pairings (owner_user_id, status, updated_at DESC);
CREATE INDEX idx_im_pairings_owner_channel_status ON im_pairings (owner_user_id, channel_type, status);
CREATE INDEX idx_im_pairings_agent ON im_pairings (agent_id, status);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_im_pairings_agent;
DROP INDEX IF EXISTS idx_im_pairings_owner_channel_status;
DROP INDEX IF EXISTS idx_im_pairings_owner_status;
DROP INDEX IF EXISTS idx_im_channel_configs_owner_status;

CREATE TABLE im_channel_configs_new (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'configured',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type),
    CONSTRAINT ck_im_channel_configs_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_channel_configs_status CHECK (status IN ('configured', 'connected', 'pending', 'error', 'disabled')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE RESTRICT
);

INSERT INTO im_channel_configs_new (
    owner_user_id, channel_type, agent_id, status, config_json,
    credentials_encrypted, last_error, created_at, updated_at
)
SELECT
    owner_user_id,
    channel_type, agent_id, status, config_json,
    credentials_encrypted, last_error, created_at, updated_at
FROM im_channel_configs;

DROP TABLE im_channel_configs;
ALTER TABLE im_channel_configs_new RENAME TO im_channel_configs;

CREATE TABLE im_pairings_new (
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

INSERT INTO im_pairings_new (
    pairing_id, owner_user_id, channel_type, chat_type, external_ref, thread_id,
    external_name, agent_id, status, source, last_message_at, created_at, updated_at
)
SELECT
    pairing_id,
    owner_user_id,
    channel_type, chat_type, external_ref, thread_id,
    external_name, agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings;

DROP TABLE im_pairings;
ALTER TABLE im_pairings_new RENAME TO im_pairings;

CREATE INDEX idx_im_channel_configs_owner_status ON im_channel_configs (owner_user_id, status, channel_type);
CREATE INDEX idx_im_pairings_owner_status ON im_pairings (owner_user_id, status, updated_at DESC);
CREATE INDEX idx_im_pairings_owner_channel_status ON im_pairings (owner_user_id, channel_type, status);
CREATE INDEX idx_im_pairings_agent ON im_pairings (agent_id, status);

PRAGMA foreign_keys = ON;
