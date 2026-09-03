-- +goose Up
CREATE TABLE local_owner_bindings (
    deployment_id TEXT NOT NULL,
    control_user_id TEXT NOT NULL,
    local_owner_key TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (deployment_id, control_user_id),
    UNIQUE (local_owner_key),
    FOREIGN KEY (local_owner_key) REFERENCES users (user_id) ON DELETE RESTRICT
);

-- +goose Down
DROP TABLE local_owner_bindings;
