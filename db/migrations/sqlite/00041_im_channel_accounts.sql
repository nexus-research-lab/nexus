-- +goose Up
CREATE TABLE IF NOT EXISTS im_channel_accounts (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    account_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'connected',
    config_json TEXT NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type, account_id),
    CONSTRAINT ck_im_channel_accounts_channel_type CHECK (channel_type IN ('dingtalk', 'wechat', 'weixin-personal', 'feishu', 'telegram', 'discord')),
    CONSTRAINT ck_im_channel_accounts_status CHECK (status IN ('configured', 'connected', 'pending', 'error', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_im_channel_accounts_owner_channel_status
ON im_channel_accounts (owner_user_id, channel_type, status);

-- +goose Down
DROP INDEX IF EXISTS idx_im_channel_accounts_owner_channel_status;
DROP TABLE IF EXISTS im_channel_accounts;
