-- +goose Up
-- =====================================================
-- @File   ：00004_provider_and_task_source.sql
-- @Date   ：2026/04/16 20:35:00
-- @Author ：leemysw
-- 2026/04/16 20:35:00   Create
-- =====================================================

ALTER TABLE runtimes ADD COLUMN provider VARCHAR(128);

CREATE TABLE provider_configs (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    auth_token TEXT NOT NULL,
    base_url TEXT NOT NULL,
    model VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL,
    is_default BOOLEAN NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);
CREATE UNIQUE INDEX uq_provider_configs_provider ON provider_configs (provider);

ALTER TABLE automation_cron_jobs ADD COLUMN source_kind VARCHAR(32) NOT NULL DEFAULT 'system';
ALTER TABLE automation_cron_jobs ADD COLUMN source_creator_agent_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN source_context_type VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN source_context_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN source_context_label VARCHAR(255);
ALTER TABLE automation_cron_jobs ADD COLUMN source_session_key VARCHAR(255);
ALTER TABLE automation_cron_jobs ADD COLUMN source_session_label VARCHAR(255);

-- +goose Down
ALTER TABLE automation_cron_jobs
    DROP COLUMN IF EXISTS source_session_label,
    DROP COLUMN IF EXISTS source_session_key,
    DROP COLUMN IF EXISTS source_context_label,
    DROP COLUMN IF EXISTS source_context_id,
    DROP COLUMN IF EXISTS source_context_type,
    DROP COLUMN IF EXISTS source_creator_agent_id,
    DROP COLUMN IF EXISTS source_kind;

DROP INDEX IF EXISTS uq_provider_configs_provider;
DROP TABLE IF EXISTS provider_configs;

ALTER TABLE runtimes DROP COLUMN IF EXISTS provider;
