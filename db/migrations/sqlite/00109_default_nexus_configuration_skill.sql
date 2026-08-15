-- +goose Up
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(json_group_array(value), '[]')
    FROM json_each(runtimes.skill_ids_json)
    WHERE LOWER(TRIM(CAST(value AS TEXT))) NOT IN (
        'nexus-configuration',
        'nexus-owner-configuration',
        'nexus-agent-self-configuration',
        'nexus-room-host-configuration',
        'nexus-room-member-configuration'
    )
)
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array';

UPDATE runtimes
SET skill_ids_json = json_insert(skill_ids_json, '$[#]', 'nexus-configuration')
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array';

-- +goose Down
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(json_group_array(value), '[]')
    FROM json_each(runtimes.skill_ids_json)
    WHERE LOWER(TRIM(CAST(value AS TEXT))) <> 'nexus-configuration'
)
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array';
