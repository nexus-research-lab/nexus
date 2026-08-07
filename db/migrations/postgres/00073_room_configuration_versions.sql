-- +goose Up
-- Room configuration CAS and authority revocation generation.
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS configuration_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS authority_epoch BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE rooms DROP COLUMN IF EXISTS authority_epoch;
ALTER TABLE rooms DROP COLUMN IF EXISTS configuration_version;
