-- +goose Up
ALTER TABLE owner_entitlements
    ADD COLUMN control_unavailable BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE owner_entitlements
    DROP COLUMN control_unavailable;
