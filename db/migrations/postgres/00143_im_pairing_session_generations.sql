-- +goose Up
ALTER TABLE im_pairings ADD COLUMN IF NOT EXISTS session_key TEXT NOT NULL DEFAULT '';
ALTER TABLE im_pairings ADD COLUMN IF NOT EXISTS session_materialized BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE im_pairings
SET session_key = 'agent:' || agent_id || ':' ||
    CASE channel_type
      WHEN 'dingtalk' THEN 'dt'
      WHEN 'wechat' THEN 'wx'
      WHEN 'weixin-personal' THEN 'weixin-personal'
      WHEN 'feishu' THEN 'fs'
      WHEN 'telegram' THEN 'tg'
      WHEN 'discord' THEN 'dg'
      ELSE channel_type
    END || ':' || chat_type || ':' ||
    CASE WHEN account_id <> '' THEN 'acct:' || account_id || ':' ELSE '' END ||
    external_ref || CASE WHEN thread_id <> '' THEN ':topic:' || thread_id ELSE '' END
WHERE session_key = '';
CREATE TABLE IF NOT EXISTS im_pairing_sessions (
    owner_user_id VARCHAR(64) NOT NULL,
    pairing_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    account_id VARCHAR(255) NOT NULL DEFAULT '',
    chat_type VARCHAR(16) NOT NULL,
    external_ref VARCHAR(255) NOT NULL,
    thread_id VARCHAR(255) NOT NULL DEFAULT '',
    session_key TEXT NOT NULL,
    session_materialized BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id),
    UNIQUE (owner_user_id, session_key)
);
CREATE INDEX IF NOT EXISTS idx_im_pairing_sessions_pairing ON im_pairing_sessions(owner_user_id, pairing_id);

-- +goose Down
DROP INDEX IF EXISTS idx_im_pairing_sessions_pairing;
DROP TABLE IF EXISTS im_pairing_sessions;
ALTER TABLE im_pairings DROP COLUMN IF EXISTS session_materialized;
ALTER TABLE im_pairings DROP COLUMN IF EXISTS session_key;
