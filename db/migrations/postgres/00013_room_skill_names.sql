-- +goose Up
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS skill_names TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE rooms DROP COLUMN IF EXISTS skill_names;
