-- +goose Up
UPDATE runtimes
SET skill_ids_json = (skill_ids_json::jsonb || '["visualize"]'::jsonb)::text
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
      WHERE LOWER(TRIM(skill.value)) = 'visualize'
  );

-- +goose Down
UPDATE runtimes
SET skill_ids_json = COALESCE((
    SELECT jsonb_agg(skill.value)::text
    FROM jsonb_array_elements_text(runtimes.skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value)) <> 'visualize'
), '[]')
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array';
