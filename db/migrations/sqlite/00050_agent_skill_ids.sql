-- +goose Up
ALTER TABLE runtimes ADD COLUMN skill_ids_json TEXT NOT NULL DEFAULT '[]';

UPDATE runtimes
SET skill_ids_json = '["imagegen","goal-manager"]';

UPDATE runtimes
SET skill_ids_json = '["imagegen","goal-manager","nexus-manager"]'
WHERE agent_id IN (SELECT id FROM agents WHERE is_main = 1);

-- +goose Down
ALTER TABLE runtimes DROP COLUMN skill_ids_json;
