-- +goose Up
-- Room configuration CAS and authority revocation generation.
ALTER TABLE rooms ADD COLUMN configuration_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE rooms ADD COLUMN authority_epoch INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE rooms DROP COLUMN authority_epoch;
ALTER TABLE rooms DROP COLUMN configuration_version;
