-- +goose Up
CREATE TABLE local_owner_bindings (
    deployment_id VARCHAR(128) NOT NULL,
    control_user_id VARCHAR(128) NOT NULL,
    local_owner_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    PRIMARY KEY (deployment_id, control_user_id),
    UNIQUE (local_owner_key),
    FOREIGN KEY (local_owner_key) REFERENCES users (user_id) ON DELETE RESTRICT
);

-- +goose Down
DROP TABLE local_owner_bindings;
