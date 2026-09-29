-- +goose Up
ALTER TABLE team_node_jobs ADD COLUMN source_room_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_node_jobs ADD COLUMN source_message_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_node_jobs ADD COLUMN delivery_id TEXT NOT NULL DEFAULT '';
UPDATE team_node_jobs SET source_room_id=COALESCE(data_json::jsonb->'Delivery'->>'room_id', ''),
 source_message_id=COALESCE(data_json::jsonb->'Delivery'->>'message_id', ''), delivery_id=COALESCE(data_json::jsonb->'Delivery'->>'id', '');
CREATE INDEX team_node_jobs_message ON team_node_jobs(owner_user_id, scope, source_room_id, source_message_id);
CREATE INDEX team_node_jobs_delivery ON team_node_jobs(owner_user_id, scope, source_room_id, delivery_id);

-- +goose Down
DROP INDEX team_node_jobs_message;
DROP INDEX team_node_jobs_delivery;
ALTER TABLE team_node_jobs DROP COLUMN delivery_id;
ALTER TABLE team_node_jobs DROP COLUMN source_message_id;
ALTER TABLE team_node_jobs DROP COLUMN source_room_id;
