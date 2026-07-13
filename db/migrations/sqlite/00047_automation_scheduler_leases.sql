-- +goose Up
CREATE TABLE automation_scheduler_leases (
    lease_name VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_automation_scheduler_leases_expires_at
    ON automation_scheduler_leases (expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_scheduler_leases_expires_at;
DROP TABLE IF EXISTS automation_scheduler_leases;
