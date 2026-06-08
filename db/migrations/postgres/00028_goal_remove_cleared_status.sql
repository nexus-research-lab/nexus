-- +goose Up

DELETE FROM session_goals
WHERE status = 'cleared';

ALTER TABLE session_goals
    DROP CONSTRAINT IF EXISTS ck_session_goals_status;

ALTER TABLE session_goals
    ADD CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'budget_limited', 'usage_limited'));

ALTER TABLE session_goals
    DROP COLUMN IF EXISTS cleared_at;

-- +goose Down

ALTER TABLE session_goals
    ADD COLUMN IF NOT EXISTS cleared_at TIMESTAMP WITHOUT TIME ZONE;

ALTER TABLE session_goals
    DROP CONSTRAINT IF EXISTS ck_session_goals_status;

ALTER TABLE session_goals
    ADD CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'budget_limited', 'usage_limited', 'cleared'));
