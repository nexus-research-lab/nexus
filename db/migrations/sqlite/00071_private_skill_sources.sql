-- +goose Up
ALTER TABLE skill_sources ADD COLUMN managed_by VARCHAR(32) NOT NULL DEFAULT 'system';
ALTER TABLE skill_sources ADD COLUMN auth_type VARCHAR(32) NOT NULL DEFAULT 'none';
ALTER TABLE skill_sources ADD COLUMN credentials_encrypted TEXT NOT NULL DEFAULT '';

ALTER TABLE imported_skills ADD COLUMN source_skill_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE imported_skills ADD COLUMN artifact_sha256 VARCHAR(64) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE imported_skills DROP COLUMN artifact_sha256;
ALTER TABLE imported_skills DROP COLUMN source_skill_id;

ALTER TABLE skill_sources DROP COLUMN credentials_encrypted;
ALTER TABLE skill_sources DROP COLUMN auth_type;
ALTER TABLE skill_sources DROP COLUMN managed_by;
