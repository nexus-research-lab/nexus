-- +goose Up
UPDATE runtimes
SET skill_ids_json = json_insert(skill_ids_json, '$[#]', 'nexus-product-guide')
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array'
  AND NOT EXISTS (
      SELECT 1
      FROM json_each(runtimes.skill_ids_json)
      WHERE LOWER(TRIM(CAST(value AS TEXT))) = 'nexus-product-guide'
  );

-- +goose Down
UPDATE runtimes
SET skill_ids_json = (
    SELECT COALESCE(json_group_array(value), '[]')
    FROM json_each(runtimes.skill_ids_json)
    WHERE LOWER(TRIM(CAST(value AS TEXT))) <> 'nexus-product-guide'
)
WHERE json_valid(skill_ids_json)
  AND json_type(skill_ids_json) = 'array';
