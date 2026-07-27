-- +goose Up

ALTER TABLE session_goals
ADD COLUMN IF NOT EXISTS usage_finalized BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE session_goals
ADD COLUMN IF NOT EXISTS usage_finalized_at TIMESTAMP WITHOUT TIME ZONE;

-- +goose Down

ALTER TABLE session_goals DROP COLUMN IF EXISTS usage_finalized_at;
ALTER TABLE session_goals DROP COLUMN IF EXISTS usage_finalized;
