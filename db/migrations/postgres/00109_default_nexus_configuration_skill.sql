-- +goose Up
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(jsonb_agg(skill.value), '[]'::jsonb)::text
    FROM jsonb_array_elements(skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value #>> '{}')) NOT IN (
        'nexus-configuration',
        'nexus-owner-configuration',
        'nexus-agent-self-configuration',
        'nexus-room-host-configuration',
        'nexus-room-member-configuration'
    )
)
WHERE skill_ids_json::jsonb IS NOT NULL
  AND jsonb_typeof(skill_ids_json::jsonb) = 'array';

UPDATE runtimes
SET skill_ids_json = (skill_ids_json::jsonb || '["nexus-configuration"]'::jsonb)::text
WHERE jsonb_typeof(skill_ids_json::jsonb) = 'array';

-- +goose Down
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(jsonb_agg(skill.value), '[]'::jsonb)::text
    FROM jsonb_array_elements(skill_ids_json::jsonb) AS skill(value)
    WHERE LOWER(TRIM(skill.value #>> '{}')) <> 'nexus-configuration'
)
WHERE skill_ids_json::jsonb IS NOT NULL
  AND jsonb_typeof(skill_ids_json::jsonb) = 'array';
