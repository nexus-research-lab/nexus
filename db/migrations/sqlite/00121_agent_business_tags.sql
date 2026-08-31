-- +goose Up
ALTER TABLE agents ADD COLUMN business_tags JSON NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE agents DROP COLUMN business_tags;
