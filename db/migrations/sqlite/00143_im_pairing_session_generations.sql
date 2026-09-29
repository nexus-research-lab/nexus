-- +goose Up
ALTER TABLE im_pairings ADD COLUMN session_key TEXT NOT NULL DEFAULT '';
ALTER TABLE im_pairings ADD COLUMN session_materialized INTEGER NOT NULL DEFAULT 0;
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
CREATE TABLE im_pairing_sessions (
    owner_user_id TEXT NOT NULL,
    pairing_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_id TEXT NOT NULL DEFAULT '',
    chat_type TEXT NOT NULL,
    external_ref TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '',
    session_key TEXT NOT NULL,
    session_materialized INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id),
    UNIQUE (owner_user_id, session_key)
);
CREATE INDEX idx_im_pairing_sessions_pairing ON im_pairing_sessions(owner_user_id, pairing_id);

-- +goose Down
-- Keep the migration reversible to preserve the rollback-to-v56 contract:
-- SQLite >= 3.35 supports DROP COLUMN, matching the Postgres variant which
-- drops the pairing generation columns on rollback.
DROP INDEX idx_im_pairing_sessions_pairing;
DROP TABLE im_pairing_sessions;
ALTER TABLE im_pairings DROP COLUMN session_key;
ALTER TABLE im_pairings DROP COLUMN session_materialized;
