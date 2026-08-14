-- +goose Up

-- Releases the historical create_goal crash window where Goal metadata was
-- committed as pending before the current transient Execution was bound. A
-- correctly bound Execution is preserved and already has a durable
-- execution_goal_confirmations receipt from migration 00098.
--
-- This is a data-safe demotion to the current Goal-only product model: the
-- unbound Execution remains WorkGraph-only and can be promoted explicitly.
UPDATE session_goals AS goal
SET metadata_json = jsonb_set(
        goal.metadata_json,
        '{execution_mode}',
        '"goal_only"'::jsonb,
        true
    ) - 'execution_id'
      - 'execution_binding_state'
      - 'completion_criteria'
      - 'promotion_command',
    version = version + 1,
    updated_at = now()
WHERE goal.status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited')
  AND goal.metadata_json ->> 'execution_binding_state' = 'pending'
  AND goal.metadata_json ->> 'execution_id' IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM executions AS execution
      WHERE execution.execution_id = goal.metadata_json ->> 'execution_id'
        AND execution.goal_id = goal.goal_id
        AND execution.goal_objective_revision = COALESCE(
            CASE
                WHEN goal.metadata_json ->> 'objective_revision' ~ '^[1-9][0-9]*$'
                THEN (goal.metadata_json ->> 'objective_revision')::BIGINT
            END,
            1
        )
  );

-- +goose Down

-- A recovered Goal may already have continued independently. Reconstructing
-- the abandoned cross-domain reservation would be unsafe.
SELECT 1;
