-- +goose Up

-- Full continuation launch receipts are deliberately separate from Goal
-- metadata: prompts are server-only and launch recovery needs its own CAS.
CREATE TABLE goal_continuation_plans (
    round_id VARCHAR(128) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    objective_revision INTEGER NOT NULL,
    execution_id VARCHAR(64),
    previous_round_id VARCHAR(128),
    prompt TEXT NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(16) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME,
    claim_expires_at DATETIME,
    last_error TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    settled_at DATETIME,
    FOREIGN KEY(goal_id) REFERENCES session_goals(goal_id) ON DELETE CASCADE,
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

-- Old binaries stored only opaque round ids in Goal metadata. The missing
-- prompt/authority makes those reservations unsafe to replay, but continuation
-- count may also contain rounds that really ran. Refund only distinct non-empty
-- string reservation identities and preserve every already-consumed attempt.
UPDATE session_goals
SET continuation_count = MAX(
        continuation_count - (
            SELECT COUNT(DISTINCT TRIM(CAST(reservation.value AS TEXT)))
            FROM json_each(
                CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
                '$.continuation_reservation_round_ids'
            ) AS reservation
            WHERE reservation.type = 'text'
              AND TRIM(CAST(reservation.value AS TEXT)) <> ''
        ),
        0
    ),
    metadata_json = json_remove(
        CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
        '$.continuation_reservation_round_ids'
    ),
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE json_type(
        CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
        '$.continuation_reservation_round_ids'
    ) = 'array';

-- +goose Down

DROP INDEX IF EXISTS idx_goal_continuation_plans_due;
DROP INDEX IF EXISTS uq_goal_continuation_plans_open_goal_revision;
DROP TABLE IF EXISTS goal_continuation_plans;
