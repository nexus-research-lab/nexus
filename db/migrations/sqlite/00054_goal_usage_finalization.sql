-- +goose Up

ALTER TABLE session_goals
ADD COLUMN usage_finalized BOOLEAN NOT NULL DEFAULT 0;

ALTER TABLE session_goals
ADD COLUMN usage_finalized_at DATETIME;

-- +goose Down

ALTER TABLE session_goals DROP COLUMN usage_finalized_at;
ALTER TABLE session_goals DROP COLUMN usage_finalized;
