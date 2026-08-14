-- +goose Up

-- Full continuation launch receipts are deliberately separate from Goal
-- metadata: prompts are server-only and launch recovery needs its own CAS.
CREATE TABLE goal_continuation_plans (
    round_id VARCHAR(128) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL REFERENCES session_goals(goal_id) ON DELETE CASCADE,
    session_key VARCHAR(512) NOT NULL,
    objective_revision BIGINT NOT NULL,
    execution_id VARCHAR(64),
    previous_round_id VARCHAR(128),
    prompt TEXT NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITHOUT TIME ZONE,
    claim_expires_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    settled_at TIMESTAMP WITHOUT TIME ZONE,
    CONSTRAINT ck_goal_continuation_plans_status
        CHECK (status IN ('scheduled', 'claimed', 'started', 'settled', 'released', 'cancelled')),
    CONSTRAINT ck_goal_continuation_plans_revision CHECK (objective_revision > 0),
    CONSTRAINT ck_goal_continuation_plans_version CHECK (version > 0),
    CONSTRAINT ck_goal_continuation_plans_attempt_count CHECK (attempt_count >= 0)
);

CREATE UNIQUE INDEX uq_goal_continuation_plans_open_goal_revision
    ON goal_continuation_plans (goal_id, objective_revision)
    WHERE status IN ('scheduled', 'claimed', 'started');
CREATE INDEX idx_goal_continuation_plans_due
    ON goal_continuation_plans (status, next_attempt_at, claim_expires_at, updated_at);

-- Historical ids have no prompt/authority payload and cannot be replayed.
-- Refund only distinct non-empty string identities: continuation_count may
-- also contain rounds that really ran and must not be reset.
UPDATE session_goals AS goal
SET continuation_count = GREATEST(
        goal.continuation_count - (
            SELECT COUNT(DISTINCT btrim(reservation.value #>> '{}'))::integer
            FROM jsonb_array_elements(
                goal.metadata_json -> 'continuation_reservation_round_ids'
            ) AS reservation(value)
            WHERE jsonb_typeof(reservation.value) = 'string'
              AND btrim(reservation.value #>> '{}') <> ''
        ),
        0
    ),
    metadata_json = goal.metadata_json - 'continuation_reservation_round_ids',
    version = goal.version + 1,
    updated_at = now()
WHERE jsonb_typeof(goal.metadata_json -> 'continuation_reservation_round_ids') = 'array';

-- +goose Down

DROP INDEX IF EXISTS idx_goal_continuation_plans_due;
DROP INDEX IF EXISTS uq_goal_continuation_plans_open_goal_revision;
DROP TABLE IF EXISTS goal_continuation_plans;
