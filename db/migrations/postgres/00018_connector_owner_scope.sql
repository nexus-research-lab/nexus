-- +goose Up
ALTER TABLE connector_connections ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
UPDATE connector_connections
SET owner_user_id = COALESCE(
    (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
    (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
    '__system__'
)
WHERE owner_user_id = '__system__';

ALTER TABLE connector_connections DROP CONSTRAINT IF EXISTS connector_connections_pkey;
ALTER TABLE connector_connections ADD PRIMARY KEY (owner_user_id, connector_id);
CREATE INDEX IF NOT EXISTS idx_connector_connections_owner_state ON connector_connections (owner_user_id, state);
ALTER TABLE connector_connections ALTER COLUMN owner_user_id DROP DEFAULT;

ALTER TABLE connector_oauth_states ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
UPDATE connector_oauth_states
SET owner_user_id = COALESCE(
    (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
    (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
    '__system__'
)
WHERE owner_user_id = '__system__';
CREATE INDEX IF NOT EXISTS idx_connector_oauth_states_owner_state ON connector_oauth_states (owner_user_id, state);
CREATE INDEX IF NOT EXISTS idx_connector_oauth_states_owner_connector ON connector_oauth_states (owner_user_id, connector_id);
ALTER TABLE connector_oauth_states ALTER COLUMN owner_user_id DROP DEFAULT;

ALTER TABLE provider ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64);
ALTER TABLE provider ADD COLUMN IF NOT EXISTS visibility VARCHAR(32) NOT NULL DEFAULT 'public';

UPDATE provider
SET owner_user_id = NULL,
    visibility = 'public';

DROP INDEX IF EXISTS ix_provider_provider;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ck_provider_visibility'
    ) THEN
        ALTER TABLE provider
            ADD CONSTRAINT ck_provider_visibility CHECK (visibility IN ('public', 'private'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ck_provider_scope_owner'
    ) THEN
        ALTER TABLE provider
            ADD CONSTRAINT ck_provider_scope_owner CHECK (
                (visibility = 'public' AND owner_user_id IS NULL)
                OR (visibility = 'private' AND owner_user_id IS NOT NULL)
            );
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_public_provider ON provider (provider) WHERE visibility = 'public';
CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_owner_provider ON provider (owner_user_id, provider) WHERE visibility = 'private';
CREATE INDEX IF NOT EXISTS idx_provider_visibility_owner ON provider (visibility, owner_user_id, provider);

CREATE TABLE IF NOT EXISTS skill_sources (
    owner_user_id VARCHAR(64) NOT NULL,
    source_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    url TEXT NOT NULL,
    trust VARCHAR(32) NOT NULL DEFAULT 'community',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 100,
    last_checked_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, source_id),
    UNIQUE (owner_user_id, kind, url)
);

CREATE INDEX IF NOT EXISTS idx_skill_sources_owner_enabled ON skill_sources (owner_user_id, enabled, sort_order);

CREATE TABLE IF NOT EXISTS imported_skills (
    owner_user_id VARCHAR(64) NOT NULL,
    skill_name VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    scope VARCHAR(32) NOT NULL DEFAULT 'any',
    tags TEXT NOT NULL DEFAULT '[]',
    category_key VARCHAR(128) NOT NULL DEFAULT 'custom-imports',
    category_name VARCHAR(128) NOT NULL DEFAULT '自定义导入',
    recommendation TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    source_id VARCHAR(64) NOT NULL DEFAULT '',
    source_kind VARCHAR(32) NOT NULL DEFAULT '',
    source_ref TEXT NOT NULL DEFAULT '',
    source_name VARCHAR(255) NOT NULL DEFAULT '',
    source_trust VARCHAR(32) NOT NULL DEFAULT 'community',
    import_mode VARCHAR(32) NOT NULL DEFAULT '',
    git_url TEXT NOT NULL DEFAULT '',
    git_branch VARCHAR(255) NOT NULL DEFAULT '',
    git_path TEXT NOT NULL DEFAULT '',
    git_commit VARCHAR(128) NOT NULL DEFAULT '',
    raw_url TEXT NOT NULL DEFAULT '',
    detail_url TEXT NOT NULL DEFAULT '',
    content_hash VARCHAR(128) NOT NULL DEFAULT '',
    last_imported_at TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, skill_name)
);

CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_source ON imported_skills (owner_user_id, source_id);
CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_kind ON imported_skills (owner_user_id, source_kind);

-- +goose Down
DROP INDEX IF EXISTS idx_imported_skills_owner_kind;
DROP INDEX IF EXISTS idx_imported_skills_owner_source;
DROP TABLE IF EXISTS imported_skills;
DROP INDEX IF EXISTS idx_skill_sources_owner_enabled;
DROP TABLE IF EXISTS skill_sources;

DROP INDEX IF EXISTS idx_provider_visibility_owner;
DROP INDEX IF EXISTS uq_provider_owner_provider;
DROP INDEX IF EXISTS uq_provider_public_provider;

DELETE FROM provider WHERE visibility = 'private';

ALTER TABLE provider DROP CONSTRAINT IF EXISTS ck_provider_scope_owner;
ALTER TABLE provider DROP CONSTRAINT IF EXISTS ck_provider_visibility;
CREATE UNIQUE INDEX IF NOT EXISTS ix_provider_provider ON provider (provider);
ALTER TABLE provider DROP COLUMN IF EXISTS visibility;
ALTER TABLE provider DROP COLUMN IF EXISTS owner_user_id;

ALTER TABLE connector_oauth_states ALTER COLUMN owner_user_id SET DEFAULT '__system__';
DROP INDEX IF EXISTS idx_connector_oauth_states_owner_connector;
DROP INDEX IF EXISTS idx_connector_oauth_states_owner_state;
ALTER TABLE connector_oauth_states DROP COLUMN IF EXISTS owner_user_id;

DROP INDEX IF EXISTS idx_connector_connections_owner_state;
ALTER TABLE connector_connections DROP CONSTRAINT IF EXISTS connector_connections_pkey;

CREATE TABLE connector_connections_dedup AS
SELECT DISTINCT ON (connector_id)
    connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, created_at, updated_at
FROM connector_connections
ORDER BY connector_id, updated_at DESC;

DROP TABLE connector_connections;
ALTER TABLE connector_connections_dedup RENAME TO connector_connections;
ALTER TABLE connector_connections ALTER COLUMN connector_id SET NOT NULL;
ALTER TABLE connector_connections ALTER COLUMN state SET NOT NULL;
ALTER TABLE connector_connections ALTER COLUMN credentials SET NOT NULL;
ALTER TABLE connector_connections ALTER COLUMN auth_type SET NOT NULL;
ALTER TABLE connector_connections ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE connector_connections ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE connector_connections ADD PRIMARY KEY (connector_id);
CREATE INDEX IF NOT EXISTS idx_connector_connections_state ON connector_connections (state);
