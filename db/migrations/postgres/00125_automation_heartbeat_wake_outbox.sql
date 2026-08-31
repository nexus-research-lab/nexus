-- +goose Up
ALTER TABLE automation_system_events
    ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS intent_digest VARCHAR(64),
    ADD COLUMN IF NOT EXISTS accepted_configuration_version BIGINT,
    ADD COLUMN IF NOT EXISTS claim_token VARCHAR(128),
    ADD COLUMN IF NOT EXISTS claim_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_automation_heartbeat_wake_request
    ON automation_system_events (owner_user_id, request_id)
    WHERE event_type = 'heartbeat.wake' AND request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_automation_heartbeat_wake_due
    ON automation_system_events (status, claim_expires_at, created_at, event_id)
    WHERE event_type = 'heartbeat.wake' AND status IN ('new', 'processing');

-- +goose Down
DROP INDEX IF EXISTS idx_automation_heartbeat_wake_due;
DROP INDEX IF EXISTS uq_automation_heartbeat_wake_request;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS claim_expires_at;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS claim_token;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS accepted_configuration_version;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS intent_digest;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS request_id;
ALTER TABLE automation_system_events DROP COLUMN IF EXISTS owner_user_id;
