-- +goose Up
DROP INDEX IF EXISTS idx_connector_connections_state;

ALTER TABLE connector_connections RENAME TO connector_connections_old;

CREATE TABLE connector_connections (
    owner_user_id VARCHAR(64) NOT NULL,
    connector_id VARCHAR(128) NOT NULL,
    state VARCHAR(32) NOT NULL,
    credentials TEXT NOT NULL,
    credentials_encrypted TEXT,
    auth_type VARCHAR(32) NOT NULL,
    oauth_state VARCHAR(255),
    oauth_state_expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, connector_id)
);

INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, created_at, updated_at
)
SELECT
    COALESCE(
        (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
        (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
        '__system__'
    ),
    connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, created_at, updated_at
FROM connector_connections_old;

DROP TABLE connector_connections_old;

CREATE INDEX idx_connector_connections_state ON connector_connections (state);
CREATE INDEX idx_connector_connections_owner_state ON connector_connections (owner_user_id, state);

ALTER TABLE connector_oauth_states ADD COLUMN owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
UPDATE connector_oauth_states
SET owner_user_id = COALESCE(
    (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
    (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
    '__system__'
)
WHERE owner_user_id = '__system__';
CREATE INDEX idx_connector_oauth_states_owner_state ON connector_oauth_states (owner_user_id, state);
CREATE INDEX idx_connector_oauth_states_owner_connector ON connector_oauth_states (owner_user_id, connector_id);

DROP INDEX IF EXISTS ix_provider_provider;

ALTER TABLE provider ADD COLUMN owner_user_id VARCHAR(64);
ALTER TABLE provider ADD COLUMN visibility VARCHAR(32) NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private'));

UPDATE provider
SET owner_user_id = NULL,
    visibility = 'public';

CREATE UNIQUE INDEX uq_provider_public_provider ON provider (provider) WHERE visibility = 'public';
CREATE UNIQUE INDEX uq_provider_owner_provider ON provider (owner_user_id, provider) WHERE visibility = 'private';
CREATE INDEX idx_provider_visibility_owner ON provider (visibility, owner_user_id, provider);

CREATE TABLE IF NOT EXISTS skill_sources (
    owner_user_id VARCHAR(64) NOT NULL,
    source_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    url TEXT NOT NULL,
    trust VARCHAR(32) NOT NULL DEFAULT 'community',
    enabled BOOLEAN NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 100,
    last_checked_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
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
    last_imported_at DATETIME,
    last_checked_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
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

CREATE UNIQUE INDEX ix_provider_provider ON provider (provider);
ALTER TABLE provider DROP COLUMN visibility;
ALTER TABLE provider DROP COLUMN owner_user_id;

DROP INDEX IF EXISTS idx_connector_oauth_states_owner_connector;
DROP INDEX IF EXISTS idx_connector_oauth_states_owner_state;

ALTER TABLE connector_oauth_states RENAME TO connector_oauth_states_old;

CREATE TABLE connector_oauth_states (
    state VARCHAR(64) NOT NULL PRIMARY KEY,
    connector_id VARCHAR(128) NOT NULL,
    code_verifier VARCHAR(128),
    redirect_uri VARCHAR(512) NOT NULL,
    shop_domain VARCHAR(255),
    extra_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    expires_at DATETIME NOT NULL,
    redirect_kind VARCHAR(32) NOT NULL DEFAULT 'web'
);

INSERT INTO connector_oauth_states (
    state, connector_id, code_verifier, redirect_uri, shop_domain, extra_json,
    created_at, expires_at, redirect_kind
)
SELECT
    state, connector_id, code_verifier, redirect_uri, shop_domain, extra_json,
    created_at, expires_at, redirect_kind
FROM connector_oauth_states_old;

DROP TABLE connector_oauth_states_old;
CREATE INDEX idx_connector_oauth_states_connector ON connector_oauth_states (connector_id);
CREATE INDEX idx_connector_oauth_states_expires ON connector_oauth_states (expires_at);
CREATE INDEX idx_connector_oauth_states_redirect_kind ON connector_oauth_states (redirect_kind);

DROP INDEX IF EXISTS idx_connector_connections_owner_state;
DROP INDEX IF EXISTS idx_connector_connections_state;

ALTER TABLE connector_connections RENAME TO connector_connections_new;

CREATE TABLE connector_connections (
    connector_id VARCHAR(128) NOT NULL PRIMARY KEY,
    state VARCHAR(32) NOT NULL,
    credentials TEXT NOT NULL,
    credentials_encrypted TEXT,
    auth_type VARCHAR(32) NOT NULL,
    oauth_state VARCHAR(255),
    oauth_state_expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);

INSERT INTO connector_connections (
    connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, created_at, updated_at
)
SELECT
    connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, created_at, updated_at
FROM connector_connections_new
GROUP BY connector_id;

DROP TABLE connector_connections_new;
CREATE INDEX idx_connector_connections_state ON connector_connections (state);
