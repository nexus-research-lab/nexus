-- +goose Up
ALTER TABLE executions
    DROP CONSTRAINT ck_executions_goal_activation_reason;

ALTER TABLE executions
    ADD CONSTRAINT ck_executions_goal_activation_reason
    CHECK (
        goal_activation_reason IS NULL
        OR goal_activation_reason IN (
            'persistence_requested', 'observed_boundary', 'room_dependency_chain',
            'external_wait', 'scheduled_retry', 'context_boundary', 'recovery_required',
            'substantial_complexity'
        )
    );

-- +goose Down
-- The new enum value may already be durable data, so narrowing this CHECK is unsafe.
SELECT 1;
