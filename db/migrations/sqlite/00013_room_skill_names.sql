-- +goose Up
ALTER TABLE rooms ADD COLUMN skill_names TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE rooms DROP COLUMN skill_names;
