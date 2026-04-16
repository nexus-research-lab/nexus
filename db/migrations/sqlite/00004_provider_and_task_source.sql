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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
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
CREATE TABLE automation_cron_jobs__rollback AS
SELECT
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    enabled,
    created_at,
    updated_at
FROM automation_cron_jobs;

DROP TABLE automation_cron_jobs;

CREATE TABLE automation_cron_jobs (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
    session_target_kind VARCHAR(32) NOT NULL,
    bound_session_key VARCHAR(255),
    named_session_key VARCHAR(255),
    wake_mode VARCHAR(32) NOT NULL,
    delivery_mode VARCHAR(32) NOT NULL,
    delivery_channel VARCHAR(64),
    delivery_to VARCHAR(255),
    delivery_account_id VARCHAR(64),
    delivery_thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_jobs_schedule_kind CHECK (schedule_kind IN ('every', 'cron', 'at')),
    CONSTRAINT ck_automation_cron_jobs_session_target_kind CHECK (session_target_kind IN ('isolated', 'main', 'bound', 'named')),
    CONSTRAINT ck_automation_cron_jobs_wake_mode CHECK (wake_mode IN ('now', 'next-heartbeat')),
    CONSTRAINT ck_automation_cron_jobs_delivery_mode CHECK (delivery_mode IN ('none', 'last', 'explicit')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX idx_automation_cron_jobs_agent ON automation_cron_jobs (agent_id);

INSERT INTO automation_cron_jobs (
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    enabled,
    created_at,
    updated_at
)
SELECT
    job_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    enabled,
    created_at,
    updated_at
FROM automation_cron_jobs__rollback;

DROP TABLE automation_cron_jobs__rollback;

DROP INDEX IF EXISTS uq_provider_configs_provider;
DROP TABLE IF EXISTS provider_configs;

CREATE TABLE runtimes__rollback AS
SELECT
    id,
    agent_id,
    model,
    permission_mode,
    allowed_tools_json,
    disallowed_tools_json,
    mcp_servers_json,
    max_turns,
    max_thinking_tokens,
    setting_sources_json,
    runtime_version,
    created_at,
    updated_at
FROM runtimes;

DROP TABLE runtimes;

CREATE TABLE runtimes (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL UNIQUE,
    model VARCHAR(128),
    permission_mode VARCHAR(64),
    allowed_tools_json TEXT NOT NULL,
    disallowed_tools_json TEXT NOT NULL,
    mcp_servers_json TEXT NOT NULL,
    max_turns INTEGER,
    max_thinking_tokens INTEGER,
    setting_sources_json TEXT NOT NULL,
    runtime_version INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT INTO runtimes (
    id,
    agent_id,
    model,
    permission_mode,
    allowed_tools_json,
    disallowed_tools_json,
    mcp_servers_json,
    max_turns,
    max_thinking_tokens,
    setting_sources_json,
    runtime_version,
    created_at,
    updated_at
)
SELECT
    id,
    agent_id,
    model,
    permission_mode,
    allowed_tools_json,
    disallowed_tools_json,
    mcp_servers_json,
    max_turns,
    max_thinking_tokens,
    setting_sources_json,
    runtime_version,
    created_at,
    updated_at
FROM runtimes__rollback;

DROP TABLE runtimes__rollback;
