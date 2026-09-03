-- +goose Up
CREATE TABLE owner_profiles (
    owner_user_id VARCHAR(128) NOT NULL PRIMARY KEY,
    username VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    avatar VARCHAR(255),
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT ck_owner_profiles_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT ck_owner_profiles_status CHECK (status IN ('active', 'disabled'))
);

INSERT INTO owner_profiles (
    owner_user_id, username, display_name, role, status, avatar, created_at, updated_at
)
SELECT user_id, username, display_name, role, status, avatar, created_at, updated_at
FROM users;

ALTER TABLE local_owner_bindings
    DROP CONSTRAINT IF EXISTS local_owner_bindings_local_owner_key_fkey;
ALTER TABLE local_owner_bindings
    ADD CONSTRAINT fk_local_owner_bindings_profile
    FOREIGN KEY (local_owner_key) REFERENCES owner_profiles (owner_user_id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE local_owner_bindings
    DROP CONSTRAINT IF EXISTS fk_local_owner_bindings_profile;

INSERT INTO users (
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
FROM owner_profiles
ON CONFLICT (user_id) DO NOTHING;

ALTER TABLE local_owner_bindings
    ADD CONSTRAINT local_owner_bindings_local_owner_key_fkey
    FOREIGN KEY (local_owner_key) REFERENCES users (user_id) ON DELETE RESTRICT;
DROP TABLE owner_profiles;
