-- +goose Up
ALTER TABLE agents ADD COLUMN IF NOT EXISTS business_tags JSON NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE agents DROP COLUMN IF EXISTS business_tags;
