-- +goose Up
-- Preserve only the host-auth database identity needed to revalidate a queued
-- direct-user request. The browser/desktop credential itself never enters the
-- Agent-writable queue.
ALTER TABLE configuration_queue_admissions
    ADD COLUMN principal_auth_method TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_queue_admissions
    ADD COLUMN principal_auth_session_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE configuration_queue_admissions
    DROP COLUMN principal_auth_session_id;
ALTER TABLE configuration_queue_admissions
    DROP COLUMN principal_auth_method;
