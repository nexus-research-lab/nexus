-- +goose Up
ALTER TABLE automation_scheduled_tasks ADD COLUMN delivery_agent_id VARCHAR(64);

UPDATE automation_scheduled_tasks
SET delivery_agent_id = (
    SELECT TRIM(r.host_agent_id)
    FROM conversations AS c
    JOIN rooms AS r ON r.id = c.room_id
    WHERE 'room:group:' || c.id = COALESCE(
        NULLIF(TRIM(automation_scheduled_tasks.delivery_session_key), ''),
        TRIM(automation_scheduled_tasks.delivery_to)
    )
      AND LOWER(TRIM(r.room_type)) = 'room'
      AND COALESCE(TRIM(r.host_agent_id), '') <> ''
    LIMIT 1
)
WHERE COALESCE(
    NULLIF(TRIM(delivery_session_key), ''),
    TRIM(delivery_to)
) LIKE 'room:group:%';

-- +goose Down
ALTER TABLE automation_scheduled_tasks DROP COLUMN delivery_agent_id;
