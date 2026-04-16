-- +goose Up
-- =====================================================
-- @File   ：00001_initial.sql
-- @Date   ：2026/04/10 21:22:41
-- @Author ：leemysw
-- 2026/04/10 21:22:41   Create
-- =====================================================

CREATE TABLE agents (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    slug VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL,
    definition TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    workspace_path VARCHAR(512) NOT NULL,
    avatar VARCHAR(255),
    vibe_tags JSON,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_agents_status CHECK (status IN ('active', 'archived', 'disabled'))
);
CREATE UNIQUE INDEX ix_agents_slug ON agents (slug);

CREATE TABLE automation_system_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    source_type VARCHAR(64),
    source_id VARCHAR(64),
    payload JSON NOT NULL,
    status VARCHAR(32) NOT NULL,
    processed_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_system_events_status CHECK (status IN ('new', 'processing', 'processed', 'failed'))
);
CREATE INDEX idx_automation_system_events_type ON automation_system_events (event_type);
CREATE INDEX idx_automation_system_events_status ON automation_system_events (status);
CREATE INDEX idx_automation_system_events_created ON automation_system_events (created_at);

CREATE TABLE auth_sessions (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_token_hash VARCHAR(64) NOT NULL,
    username VARCHAR(128) NOT NULL,
    expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);
CREATE INDEX idx_auth_sessions_username ON auth_sessions (username);
CREATE UNIQUE INDEX uq_auth_sessions_token_hash ON auth_sessions (session_token_hash);

CREATE TABLE rooms (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    room_type VARCHAR(32) NOT NULL,
    name VARCHAR(128),
    description TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_rooms_type CHECK (room_type IN ('dm', 'room'))
);
CREATE INDEX ix_rooms_room_type ON rooms (room_type);

CREATE TABLE connector_connections (
    connector_id VARCHAR(128) NOT NULL PRIMARY KEY,
    state VARCHAR(32) NOT NULL,
    credentials TEXT NOT NULL,
    auth_type VARCHAR(32) NOT NULL,
    oauth_state VARCHAR(255),
    oauth_state_expires_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);

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
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_cron_jobs_schedule_kind CHECK (schedule_kind IN ('every', 'cron', 'at')),
    CONSTRAINT ck_automation_cron_jobs_session_target_kind CHECK (session_target_kind IN ('isolated', 'main', 'bound', 'named')),
    CONSTRAINT ck_automation_cron_jobs_wake_mode CHECK (wake_mode IN ('now', 'next-heartbeat')),
    CONSTRAINT ck_automation_cron_jobs_delivery_mode CHECK (delivery_mode IN ('none', 'last', 'explicit')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX idx_automation_cron_jobs_agent ON automation_cron_jobs (agent_id);

CREATE TABLE automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_delivery_routes_mode CHECK (mode IN ('none', 'last', 'explicit')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX idx_automation_delivery_routes_agent ON automation_delivery_routes (agent_id);

CREATE TABLE automation_heartbeat_states (
    state_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL,
    every_seconds INTEGER NOT NULL,
    target_mode VARCHAR(32) NOT NULL,
    ack_max_chars INTEGER NOT NULL,
    last_heartbeat_at TIMESTAMP WITHOUT TIME ZONE,
    last_ack_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_heartbeat_states_target_mode CHECK (target_mode IN ('none', 'last', 'explicit')),
    CONSTRAINT uq_automation_heartbeat_states_agent UNIQUE (agent_id),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

CREATE TABLE contacts (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_agent_id VARCHAR(64) NOT NULL,
    contact_agent_id VARCHAR(64) NOT NULL,
    alias VARCHAR(128),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    FOREIGN KEY(owner_agent_id) REFERENCES agents (id) ON DELETE CASCADE,
    FOREIGN KEY(contact_agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_contacts_agent ON contacts (owner_agent_id, contact_agent_id);

CREATE TABLE conversations (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    room_id VARCHAR(64) NOT NULL,
    conversation_type VARCHAR(32) NOT NULL,
    title VARCHAR(255),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_conversations_type CHECK (conversation_type IN ('dm', 'room_main', 'topic')),
    FOREIGN KEY(room_id) REFERENCES rooms (id) ON DELETE CASCADE
);
CREATE INDEX idx_conversations_room ON conversations (room_id, created_at);

CREATE TABLE members (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    room_id VARCHAR(64) NOT NULL,
    member_type VARCHAR(32) NOT NULL,
    member_user_id VARCHAR(64),
    member_agent_id VARCHAR(64),
    joined_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_members_target CHECK ((member_type = 'agent' AND member_agent_id IS NOT NULL AND member_user_id IS NULL) OR (member_type = 'user' AND member_user_id IS NOT NULL AND member_agent_id IS NULL)),
    FOREIGN KEY(room_id) REFERENCES rooms (id) ON DELETE CASCADE,
    FOREIGN KEY(member_agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_members_agent ON members (room_id, member_agent_id) WHERE member_type = 'agent' AND member_agent_id IS NOT NULL;
CREATE UNIQUE INDEX uq_members_user ON members (room_id, member_user_id) WHERE member_type = 'user' AND member_user_id IS NOT NULL;

CREATE TABLE profiles (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL,
    avatar_url VARCHAR(512),
    headline VARCHAR(255) NOT NULL,
    profile_markdown TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

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
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

CREATE TABLE automation_cron_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    scheduled_for TIMESTAMP WITHOUT TIME ZONE,
    started_at TIMESTAMP WITHOUT TIME ZONE,
    finished_at TIMESTAMP WITHOUT TIME ZONE,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_cron_runs_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    FOREIGN KEY(job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE
);
CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);

CREATE TABLE sessions (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    conversation_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(64) NOT NULL,
    version_no INTEGER NOT NULL,
    branch_key VARCHAR(128) NOT NULL,
    is_primary BOOLEAN NOT NULL,
    sdk_session_id VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    last_activity_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_sessions_status CHECK (status IN ('active', 'idle', 'interrupted', 'closed')),
    FOREIGN KEY(conversation_id) REFERENCES conversations (id) ON DELETE CASCADE,
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE,
    FOREIGN KEY(runtime_id) REFERENCES runtimes (id) ON DELETE CASCADE
);
CREATE INDEX idx_sessions_conversation ON sessions (conversation_id, last_activity_at);
CREATE UNIQUE INDEX uq_sessions_branch ON sessions (conversation_id, agent_id, branch_key);
CREATE INDEX idx_sessions_agent ON sessions (agent_id, last_activity_at);
CREATE UNIQUE INDEX uq_sessions_primary ON sessions (conversation_id, agent_id) WHERE is_primary;

CREATE TABLE messages (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    conversation_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64),
    sender_type VARCHAR(32) NOT NULL,
    sender_user_id VARCHAR(64),
    sender_agent_id VARCHAR(64),
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    content_preview TEXT,
    jsonl_path VARCHAR(512) NOT NULL,
    jsonl_offset INTEGER,
    round_id VARCHAR(64),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_messages_sender_type CHECK (sender_type IN ('user', 'agent', 'system', 'tool')),
    CONSTRAINT ck_messages_kind CHECK (kind IN ('text', 'tool_call', 'tool_result', 'event', 'error')),
    CONSTRAINT ck_messages_status CHECK (status IN ('pending', 'streaming', 'completed', 'cancelled', 'error')),
    FOREIGN KEY(conversation_id) REFERENCES conversations (id) ON DELETE CASCADE,
    FOREIGN KEY(session_id) REFERENCES sessions (id) ON DELETE SET NULL,
    FOREIGN KEY(sender_agent_id) REFERENCES agents (id) ON DELETE SET NULL
);
CREATE INDEX idx_messages_conversation_status ON messages (conversation_id, status, created_at);
CREATE INDEX idx_messages_conversation ON messages (conversation_id, created_at);
CREATE INDEX idx_messages_session ON messages (session_id, created_at);

CREATE TABLE rounds (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL,
    round_id VARCHAR(64) NOT NULL UNIQUE,
    trigger_message_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    finished_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_rounds_status CHECK (status IN ('running', 'success', 'error', 'cancelled')),
    FOREIGN KEY(session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    FOREIGN KEY(trigger_message_id) REFERENCES messages (id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS rounds;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS automation_cron_runs;
DROP TABLE IF EXISTS runtimes;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS automation_heartbeat_states;
DROP TABLE IF EXISTS automation_delivery_routes;
DROP TABLE IF EXISTS automation_cron_jobs;
DROP TABLE IF EXISTS connector_connections;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS automation_system_events;
DROP TABLE IF EXISTS agents;
