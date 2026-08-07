-- +goose Up
-- Durable, scope-bound handoff between an Agent-requested Channel authorization
-- and the authenticated human UI. QR/device material stays encrypted and is
-- scrubbed as soon as a flow reaches a terminal state.
CREATE TABLE channel_authorization_flows (
    flow_id TEXT NOT NULL PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    principal_user_id TEXT NOT NULL,
    principal_role TEXT NOT NULL DEFAULT '',
    principal_auth_method TEXT NOT NULL DEFAULT '',
    principal_auth_session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL,
    business_session_key TEXT NOT NULL,
    root_round_id TEXT NOT NULL,
    runtime_lease_session_key TEXT NOT NULL,
    runtime_lease_round_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_binding TEXT NOT NULL DEFAULT 'new',
    resolved_account_id TEXT NOT NULL DEFAULT '',
    start_control_version INTEGER NOT NULL,
    committed_control_version INTEGER,
    flow_generation TEXT NOT NULL,
    process_generation TEXT NOT NULL,
    status TEXT NOT NULL,
    runtime_ref_encrypted TEXT,
    human_presentation_encrypted TEXT,
    outcome_code TEXT NOT NULL DEFAULT '',
    outcome_message TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    CONSTRAINT ck_channel_authorization_flows_status CHECK (
        status IN (
            'starting',
            'running',
            'verify_code_required',
            'succeeded',
            'error',
            'expired',
            'cancelled',
            'restart_invalidated'
        )
    ),
    CONSTRAINT ck_channel_authorization_flows_version CHECK (
        start_control_version > 0 AND
        (committed_control_version IS NULL OR committed_control_version > 0)
    )
);

CREATE UNIQUE INDEX idx_channel_authorization_flows_active
    ON channel_authorization_flows(owner_user_id, channel_type)
    WHERE status IN ('starting', 'running', 'verify_code_required');

CREATE UNIQUE INDEX idx_channel_authorization_flows_generation
    ON channel_authorization_flows(owner_user_id, channel_type, flow_generation);

CREATE INDEX idx_channel_authorization_flows_owner_status
    ON channel_authorization_flows(owner_user_id, status, updated_at);

-- One immutable, secret-free terminal record per flow. This is the stable
-- completion/audit surface consumed by the conversational control plane.
CREATE TABLE channel_authorization_audit (
    audit_id TEXT NOT NULL PRIMARY KEY,
    flow_id TEXT NOT NULL UNIQUE,
    owner_user_id TEXT NOT NULL,
    principal_user_id TEXT NOT NULL,
    principal_role TEXT NOT NULL DEFAULT '',
    principal_auth_method TEXT NOT NULL DEFAULT '',
    principal_auth_session_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL,
    business_session_key TEXT NOT NULL,
    root_round_id TEXT NOT NULL,
    runtime_lease_session_key TEXT NOT NULL,
    runtime_lease_round_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_binding TEXT NOT NULL DEFAULT 'new',
    resolved_account_id TEXT NOT NULL DEFAULT '',
    start_control_version INTEGER NOT NULL,
    committed_control_version INTEGER,
    flow_generation TEXT NOT NULL,
    status TEXT NOT NULL,
    outcome_code TEXT NOT NULL DEFAULT '',
    outcome_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_channel_authorization_audit_status CHECK (
        status IN (
            'succeeded',
            'error',
            'expired',
            'cancelled',
            'restart_invalidated'
        )
    )
);

CREATE INDEX idx_channel_authorization_audit_owner_completed
    ON channel_authorization_audit(owner_user_id, completed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_channel_authorization_audit_owner_completed;
DROP TABLE IF EXISTS channel_authorization_audit;
DROP INDEX IF EXISTS idx_channel_authorization_flows_owner_status;
DROP INDEX IF EXISTS idx_channel_authorization_flows_generation;
DROP INDEX IF EXISTS idx_channel_authorization_flows_active;
DROP TABLE IF EXISTS channel_authorization_flows;
