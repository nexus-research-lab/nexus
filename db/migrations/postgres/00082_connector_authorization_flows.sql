-- +goose Up
-- Durable, human-approved conversational OAuth/device flows. Provider secrets
-- remain in the existing OAuth state store or encrypted flow payload and are
-- never part of the Agent-facing flow identifier.
ALTER TABLE connector_oauth_states ADD COLUMN IF NOT EXISTS control_flow_id TEXT;
CREATE INDEX IF NOT EXISTS idx_connector_oauth_states_control_flow
    ON connector_oauth_states (control_flow_id);

CREATE TABLE IF NOT EXISTS connector_authorization_flows (
    flow_id TEXT NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    human_principal_user_id VARCHAR(64) NOT NULL,
    human_principal_role VARCHAR(32) NOT NULL,
    human_auth_method VARCHAR(32) NOT NULL,
    human_auth_session_id TEXT,
    permission_request_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    business_session_key TEXT NOT NULL,
    root_round_id TEXT NOT NULL,
    runtime_lease_session_key TEXT NOT NULL,
    runtime_lease_round_id TEXT NOT NULL,
    connector_id VARCHAR(128) NOT NULL,
    authorization_method VARCHAR(32) NOT NULL
        CHECK (authorization_method IN ('oauth_browser', 'device')),
    device_mode VARCHAR(32) NOT NULL DEFAULT '',
    intent_digest VARCHAR(64) NOT NULL,
    start_configuration_version BIGINT NOT NULL,
    expected_configuration_version BIGINT NOT NULL,
    completed_configuration_version BIGINT,
    status VARCHAR(32) NOT NULL
        CHECK (status IN (
            'approved', 'pending', 'polling', 'connected', 'expired',
            'denied', 'canceled', 'conflict', 'failed'
        )),
    stage VARCHAR(64) NOT NULL DEFAULT '',
    secret_encrypted TEXT NOT NULL DEFAULT '',
    public_user_code TEXT NOT NULL DEFAULT '',
    public_verification_uri TEXT NOT NULL DEFAULT '',
    public_verification_uri_complete TEXT NOT NULL DEFAULT '',
    public_open_path TEXT NOT NULL DEFAULT '',
    poll_interval_seconds INTEGER NOT NULL DEFAULT 0,
    result_message TEXT NOT NULL DEFAULT '',
    human_approved_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ,
    next_poll_at TIMESTAMPTZ,
    poll_claim_until TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_user_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_connector_authorization_flows_owner_session
    ON connector_authorization_flows (
        owner_user_id, agent_id, business_session_key, created_at
    );
CREATE INDEX IF NOT EXISTS idx_connector_authorization_flows_owner_connector
    ON connector_authorization_flows (
        owner_user_id, connector_id, status, created_at
    );
CREATE INDEX IF NOT EXISTS idx_connector_authorization_flows_expiry
    ON connector_authorization_flows (status, expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_connector_authorization_flows_expiry;
DROP INDEX IF EXISTS idx_connector_authorization_flows_owner_connector;
DROP INDEX IF EXISTS idx_connector_authorization_flows_owner_session;
DROP TABLE IF EXISTS connector_authorization_flows;
DROP INDEX IF EXISTS idx_connector_oauth_states_control_flow;
ALTER TABLE connector_oauth_states DROP COLUMN IF EXISTS control_flow_id;
