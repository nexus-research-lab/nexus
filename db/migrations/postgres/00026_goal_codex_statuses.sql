-- +goose Up

ALTER TABLE session_goals
    ADD COLUMN IF NOT EXISTS time_used_seconds BIGINT NOT NULL DEFAULT 0;

ALTER TABLE session_goals
    DROP CONSTRAINT IF EXISTS ck_session_goals_status;

ALTER TABLE session_goals
    ADD CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'budget_limited', 'usage_limited', 'cleared'));

DROP INDEX IF EXISTS uq_session_goals_current;
CREATE UNIQUE INDEX uq_session_goals_current ON session_goals (session_key) WHERE status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited');

-- +goose Down

UPDATE session_goals
SET status = 'paused',
    last_error = CASE
        WHEN COALESCE(last_error, '') = '' THEN 'Goal stopped by removed status'
        ELSE last_error
    END
WHERE status IN ('budget_limited', 'usage_limited');

DROP INDEX IF EXISTS uq_session_goals_current;

ALTER TABLE session_goals
    DROP CONSTRAINT IF EXISTS ck_session_goals_status;

ALTER TABLE session_goals
    ADD CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'cleared'));

CREATE UNIQUE INDEX uq_session_goals_current ON session_goals (session_key) WHERE status IN ('active', 'paused', 'blocked');

ALTER TABLE session_goals
    DROP COLUMN IF EXISTS time_used_seconds;
