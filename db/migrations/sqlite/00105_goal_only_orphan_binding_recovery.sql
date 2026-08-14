-- +goose Up

-- Releases the historical create_goal crash window where Goal metadata was
-- committed as pending before the current transient Execution was bound. A
-- correctly bound Execution is preserved and already has a durable
-- execution_goal_confirmations receipt from migration 00098.
--
-- This is a data-safe demotion to the current Goal-only product model: the
-- unbound Execution remains WorkGraph-only and can be promoted explicitly.
UPDATE session_goals AS goal
SET metadata_json = json_remove(
        json_set(
            CASE
                WHEN json_valid(goal.metadata_json) THEN goal.metadata_json
                ELSE '{}'
            END,
            '$.execution_mode',
            'goal_only'
        ),
        '$.execution_id',
        '$.execution_binding_state',
        '$.completion_criteria',
        '$.promotion_command'
    ),
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE goal.status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited')
  AND json_valid(goal.metadata_json)
  AND json_extract(
      CASE WHEN json_valid(goal.metadata_json) THEN goal.metadata_json ELSE '{}' END,
      '$.execution_binding_state'
  ) = 'pending'
  AND json_extract(
      CASE WHEN json_valid(goal.metadata_json) THEN goal.metadata_json ELSE '{}' END,
      '$.execution_id'
  ) IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM executions AS execution
      WHERE execution.execution_id = json_extract(
              CASE WHEN json_valid(goal.metadata_json) THEN goal.metadata_json ELSE '{}' END,
              '$.execution_id'
          )
        AND execution.goal_id = goal.goal_id
        AND execution.goal_objective_revision = COALESCE(
            CAST(json_extract(
                CASE WHEN json_valid(goal.metadata_json) THEN goal.metadata_json ELSE '{}' END,
                '$.objective_revision'
            ) AS INTEGER),
            1
        )
  );

-- +goose Down

-- A recovered Goal may already have continued independently. Reconstructing
-- the abandoned cross-domain reservation would be unsafe.
SELECT 1;
