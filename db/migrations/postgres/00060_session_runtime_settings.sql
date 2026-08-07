-- +goose Up
ALTER TABLE sessions
ADD COLUMN options_json TEXT NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE sessions
DROP COLUMN options_json;
