-- +goose Up
DROP INDEX IF EXISTS idx_imported_skills_owner_kind;
DROP INDEX IF EXISTS idx_imported_skills_owner_source;

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

CREATE TABLE imported_skills_new (
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
    update_available BOOLEAN NOT NULL DEFAULT 0,
    last_imported_at DATETIME,
    last_checked_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, skill_name)
);

INSERT INTO imported_skills_new (
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    update_available, last_imported_at, last_checked_at, last_error, created_at, updated_at
)
SELECT
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    0, last_imported_at, last_checked_at, last_error, created_at, updated_at
FROM imported_skills;

DROP TABLE imported_skills;
ALTER TABLE imported_skills_new RENAME TO imported_skills;

CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_source ON imported_skills (owner_user_id, source_id);
CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_kind ON imported_skills (owner_user_id, source_kind);

-- +goose Down
DROP INDEX IF EXISTS idx_imported_skills_owner_kind;
DROP INDEX IF EXISTS idx_imported_skills_owner_source;

CREATE TABLE imported_skills_old (
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

INSERT INTO imported_skills_old (
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    last_imported_at, last_checked_at, last_error, created_at, updated_at
)
SELECT
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    last_imported_at, last_checked_at, last_error, created_at, updated_at
FROM imported_skills;

DROP TABLE imported_skills;
ALTER TABLE imported_skills_old RENAME TO imported_skills;

CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_source ON imported_skills (owner_user_id, source_id);
CREATE INDEX IF NOT EXISTS idx_imported_skills_owner_kind ON imported_skills (owner_user_id, source_kind);
