-- +goose Up
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN IF NOT EXISTS delivery_agent_id VARCHAR(64);

UPDATE automation_scheduled_tasks AS task
SET delivery_agent_id = TRIM(room.host_agent_id)
FROM conversations AS conversation
JOIN rooms AS room ON room.id = conversation.room_id
WHERE 'room:group:' || conversation.id = COALESCE(
        NULLIF(TRIM(task.delivery_session_key), ''),
        TRIM(task.delivery_to)
    )
  AND LOWER(TRIM(room.room_type)) = 'room'
  AND COALESCE(TRIM(room.host_agent_id), '') <> ''
  AND COALESCE(
        NULLIF(TRIM(task.delivery_session_key), ''),
        TRIM(task.delivery_to)
    ) LIKE 'room:group:%';

-- +goose Down
ALTER TABLE automation_scheduled_tasks
    DROP COLUMN IF EXISTS delivery_agent_id;
