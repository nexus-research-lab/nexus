-- +goose Up
CREATE TABLE owner_profiles (
    owner_user_id TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    avatar TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT ck_owner_profiles_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT ck_owner_profiles_status CHECK (status IN ('active', 'disabled'))
);

INSERT INTO owner_profiles (
    owner_user_id, username, display_name, role, status, avatar, created_at, updated_at
)
SELECT user_id, username, display_name, role, status, avatar, created_at, updated_at
FROM users;

CREATE TABLE local_owner_bindings_next (
    deployment_id TEXT NOT NULL,
    control_user_id TEXT NOT NULL,
    local_owner_key TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (deployment_id, control_user_id),
    UNIQUE (local_owner_key),
    FOREIGN KEY (local_owner_key) REFERENCES owner_profiles (owner_user_id) ON DELETE RESTRICT
);

INSERT INTO local_owner_bindings_next (
    deployment_id, control_user_id, local_owner_key, created_at, updated_at
)
SELECT deployment_id, control_user_id, local_owner_key, created_at, updated_at
FROM local_owner_bindings;

DROP TABLE local_owner_bindings;
ALTER TABLE local_owner_bindings_next RENAME TO local_owner_bindings;

-- +goose Down
INSERT OR IGNORE INTO users (
    user_id, username, display_name, role, status, avatar, created_at, updated_at
)
SELECT
    owner_user_id,
    'legacy_' || owner_user_id,
    display_name,
    role,
    status,
    avatar,
    created_at,
    updated_at
FROM owner_profiles;

CREATE TABLE local_owner_bindings_previous (
    deployment_id TEXT NOT NULL,
    control_user_id TEXT NOT NULL,
    local_owner_key TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (deployment_id, control_user_id),
    UNIQUE (local_owner_key),
    FOREIGN KEY (local_owner_key) REFERENCES users (user_id) ON DELETE RESTRICT
);

INSERT INTO local_owner_bindings_previous (
    deployment_id, control_user_id, local_owner_key, created_at, updated_at
)
SELECT deployment_id, control_user_id, local_owner_key, created_at, updated_at
FROM local_owner_bindings;

DROP TABLE local_owner_bindings;
ALTER TABLE local_owner_bindings_previous RENAME TO local_owner_bindings;
DROP TABLE owner_profiles;
