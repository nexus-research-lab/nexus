-- +goose Up
ALTER TABLE runtimes ADD COLUMN disabled_skill_ids_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE runtimes DROP COLUMN disabled_skill_ids_json;
