-- +goose Up

DROP INDEX IF EXISTS uq_execution_attempts_room_round;
CREATE UNIQUE INDEX uq_execution_attempts_assignment_round
    ON execution_attempts (
        runtime_session_key,
        runtime_round_id,
        agent_round_id,
        assignment_id
    )
    WHERE parent_attempt_id IS NULL
      AND runtime_session_key IS NOT NULL
      AND runtime_round_id IS NOT NULL
      AND agent_round_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS uq_execution_attempts_assignment_round;
CREATE UNIQUE INDEX uq_execution_attempts_room_round
    ON execution_attempts (runtime_session_key, runtime_round_id, agent_round_id)
    WHERE parent_attempt_id IS NULL
      AND runtime_session_key IS NOT NULL
      AND runtime_round_id IS NOT NULL
      AND agent_round_id IS NOT NULL;
