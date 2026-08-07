-- +goose Up
CREATE TABLE skill_catalog_versions (
    owner_user_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS skill_catalog_versions;
