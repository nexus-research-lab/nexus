-- +goose Up
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
    WHERE LOWER(TRIM(skill.value)) <> 'execution-orchestrator'
), '[]')
WHERE jsonb_typeof(disabled_skill_ids_json::jsonb) = 'array';

ALTER TABLE token_usage_records DROP COLUMN IF EXISTS goal_tool_surface;
ALTER TABLE token_usage_records DROP COLUMN IF EXISTS execution_tool_surface;

-- +goose Down
ALTER TABLE token_usage_records ADD COLUMN IF NOT EXISTS goal_tool_surface VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN IF NOT EXISTS execution_tool_surface VARCHAR(16) NOT NULL DEFAULT 'unknown';

UPDATE runtimes
SET skill_ids_json = COALESCE((
    SELECT jsonb_agg(skill.value)::text
    FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value)) <> 'execution-orchestrator'
), '[]')
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array';
