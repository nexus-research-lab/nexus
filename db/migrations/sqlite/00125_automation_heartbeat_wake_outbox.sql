-- +goose Up
ALTER TABLE automation_system_events ADD COLUMN owner_user_id VARCHAR(64);
ALTER TABLE automation_system_events ADD COLUMN request_id VARCHAR(128);
ALTER TABLE automation_system_events ADD COLUMN intent_digest VARCHAR(64);
ALTER TABLE automation_system_events ADD COLUMN accepted_configuration_version INTEGER;
ALTER TABLE automation_system_events ADD COLUMN claim_token VARCHAR(128);
ALTER TABLE automation_system_events ADD COLUMN claim_expires_at DATETIME;

CREATE UNIQUE INDEX uq_automation_heartbeat_wake_request
    ON automation_system_events (owner_user_id, request_id)
    WHERE event_type = 'heartbeat.wake' AND request_id IS NOT NULL;
CREATE INDEX idx_automation_heartbeat_wake_due
    ON automation_system_events (status, claim_expires_at, created_at, event_id)
    WHERE event_type = 'heartbeat.wake' AND status IN ('new', 'processing');

-- +goose Down
DROP INDEX IF EXISTS idx_automation_heartbeat_wake_due;
DROP INDEX IF EXISTS uq_automation_heartbeat_wake_request;
ALTER TABLE automation_system_events DROP COLUMN claim_expires_at;
ALTER TABLE automation_system_events DROP COLUMN claim_token;
ALTER TABLE automation_system_events DROP COLUMN accepted_configuration_version;
ALTER TABLE automation_system_events DROP COLUMN intent_digest;
ALTER TABLE automation_system_events DROP COLUMN request_id;
ALTER TABLE automation_system_events DROP COLUMN owner_user_id;
