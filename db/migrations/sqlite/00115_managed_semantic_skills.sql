-- +goose Up
UPDATE runtimes
SET skill_ids_json = json_insert(skill_ids_json, '$[#]', 'goal-manager')
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM json_each(runtimes.skill_ids_json)
      WHERE LOWER(TRIM(CAST(value AS TEXT))) = 'goal-manager'
  );

UPDATE runtimes
SET skill_ids_json = json_insert(skill_ids_json, '$[#]', 'execution-orchestrator')
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM json_each(runtimes.skill_ids_json)
      WHERE LOWER(TRIM(CAST(value AS TEXT))) = 'execution-orchestrator'
  );

UPDATE runtimes
SET disabled_skill_ids_json = (
    SELECT COALESCE(json_group_array(value), '[]')
    FROM json_each(runtimes.disabled_skill_ids_json)
    WHERE LOWER(TRIM(CAST(value AS TEXT))) NOT IN ('goal-manager', 'execution-orchestrator')
)
WHERE json_valid(disabled_skill_ids_json)
  AND json_type(disabled_skill_ids_json) = 'array';

-- +goose Down
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(json_group_array(value), '[]')
    FROM json_each(runtimes.skill_ids_json)
    WHERE LOWER(TRIM(CAST(value AS TEXT))) <> 'execution-orchestrator'
)
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array';
