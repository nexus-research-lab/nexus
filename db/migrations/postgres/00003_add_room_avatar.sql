-- +goose Up
ALTER TABLE rooms ADD COLUMN avatar VARCHAR(255);

-- +goose Down
ALTER TABLE rooms DROP COLUMN IF EXISTS avatar;
