-- +goose Up

UPDATE session_goals
SET metadata_json = jsonb_set(
    metadata_json - 'completion_tool_retry_count',
    '{completion_command_retry_count}',
    metadata_json -> 'completion_tool_retry_count',
    true
)
WHERE metadata_json ? 'completion_tool_retry_count';

UPDATE session_goals
SET metadata_json = jsonb_set(metadata_json, '{created_via}', '"goal_command"'::jsonb, true)
WHERE metadata_json ->> 'created_via' = 'goal_tool';

UPDATE goal_events
SET event_type = 'completion_command_retry'
WHERE event_type = 'completion_tool_retry';

UPDATE goal_events
SET payload_json = jsonb_set(payload_json, '{source}', '"completion_command_miss"'::jsonb, true)
WHERE payload_json ->> 'source' = 'completion_tool_miss';

-- +goose Down

UPDATE session_goals
SET metadata_json = jsonb_set(
    metadata_json - 'completion_command_retry_count',
    '{completion_tool_retry_count}',
    metadata_json -> 'completion_command_retry_count',
    true
)
WHERE metadata_json ? 'completion_command_retry_count';

UPDATE session_goals
SET metadata_json = jsonb_set(metadata_json, '{created_via}', '"goal_tool"'::jsonb, true)
WHERE metadata_json ->> 'created_via' = 'goal_command';

UPDATE goal_events
SET event_type = 'completion_tool_retry'
WHERE event_type = 'completion_command_retry';

UPDATE goal_events
SET payload_json = jsonb_set(payload_json, '{source}', '"completion_tool_miss"'::jsonb, true)
WHERE payload_json ->> 'source' = 'completion_command_miss';
