-- +goose Up
CREATE TABLE IF NOT EXISTS im_channel_configs (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'configured',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type),
    CONSTRAINT ck_im_channel_configs_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_channel_configs_status CHECK (status IN ('configured', 'connected', 'pending', 'error', 'disabled')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS im_pairings (
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
    last_message_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_im_pairings_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_pairings_chat_type CHECK (chat_type IN ('dm', 'group')),
    CONSTRAINT ck_im_pairings_status CHECK (status IN ('pending', 'active', 'disabled', 'rejected')),
    CONSTRAINT ck_im_pairings_source CHECK (source IN ('manual', 'ingress', 'wechat_qr')),
    CONSTRAINT uq_im_pairings_target UNIQUE (owner_user_id, channel_type, chat_type, external_ref, thread_id),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_im_channel_configs_owner_status ON im_channel_configs (owner_user_id, status, channel_type);
CREATE INDEX IF NOT EXISTS idx_im_pairings_owner_status ON im_pairings (owner_user_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_im_pairings_owner_channel_status ON im_pairings (owner_user_id, channel_type, status);
CREATE INDEX IF NOT EXISTS idx_im_pairings_agent ON im_pairings (agent_id, status);

-- +goose Down
DROP INDEX IF EXISTS idx_im_pairings_agent;
DROP INDEX IF EXISTS idx_im_pairings_owner_channel_status;
DROP INDEX IF EXISTS idx_im_pairings_owner_status;
DROP INDEX IF EXISTS idx_im_channel_configs_owner_status;
DROP TABLE IF EXISTS im_pairings;
DROP TABLE IF EXISTS im_channel_configs;
