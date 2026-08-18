-- +goose Up
UPDATE runtimes
SET skill_ids_json = (skill_ids_json::jsonb || '["goal-manager"]'::jsonb)::text
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
      WHERE LOWER(TRIM(skill.value)) = 'goal-manager'
  );

UPDATE runtimes
SET skill_ids_json = (skill_ids_json::jsonb || '["execution-orchestrator"]'::jsonb)::text
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
      WHERE LOWER(TRIM(skill.value)) = 'execution-orchestrator'
  );

UPDATE runtimes
SET disabled_skill_ids_json = COALESCE((
    SELECT jsonb_agg(skill.value)::text
    FROM jsonb_array_elements_text(runtimes.disabled_skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value)) NOT IN ('goal-manager', 'execution-orchestrator')
), '[]')
WHERE jsonb_typeof(disabled_skill_ids_json::jsonb) = 'array';

-- +goose Down
UPDATE runtimes
SET skill_ids_json = COALESCE((
    SELECT jsonb_agg(skill.value)::text
    FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value)) <> 'execution-orchestrator'
), '[]')
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array';
