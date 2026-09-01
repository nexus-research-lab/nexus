-- +goose Up
CREATE TABLE IF NOT EXISTS auth_password_change_receipts (
    user_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    effect VARCHAR(32) NOT NULL CHECK (effect IN ('committed', 'not_applied')),
    resolved_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, request_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS auth_password_change_receipts;
