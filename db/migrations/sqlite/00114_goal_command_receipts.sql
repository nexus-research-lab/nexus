-- +goose Up

UPDATE session_goals
SET metadata_json = json_remove(
    json_set(
        CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
        '$.completion_command_retry_count',
        json_extract(
            CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
            '$.completion_tool_retry_count'
        )
    ),
    '$.completion_tool_retry_count'
)
WHERE json_type(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.completion_tool_retry_count'
) IS NOT NULL;

UPDATE session_goals
SET metadata_json = json_set(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.created_via',
    'goal_command'
)
WHERE json_extract(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.created_via'
) = 'goal_tool';

UPDATE goal_events
SET event_type = 'completion_command_retry'
WHERE event_type = 'completion_tool_retry';

UPDATE goal_events
SET payload_json = json_set(
    CASE WHEN json_valid(payload_json) THEN payload_json ELSE '{}' END,
    '$.source',
    'completion_command_miss'
)
WHERE json_extract(
    CASE WHEN json_valid(payload_json) THEN payload_json ELSE '{}' END,
    '$.source'
) = 'completion_tool_miss';

-- +goose Down

UPDATE session_goals
SET metadata_json = json_remove(
    json_set(
        CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
        '$.completion_tool_retry_count',
        json_extract(
            CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
            '$.completion_command_retry_count'
        )
    ),
    '$.completion_command_retry_count'
)
WHERE json_type(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.completion_command_retry_count'
) IS NOT NULL;

UPDATE session_goals
SET metadata_json = json_set(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.created_via',
    'goal_tool'
)
WHERE json_extract(
    CASE WHEN json_valid(metadata_json) THEN metadata_json ELSE '{}' END,
    '$.created_via'
) = 'goal_command';

UPDATE goal_events
SET event_type = 'completion_tool_retry'
WHERE event_type = 'completion_command_retry';

UPDATE goal_events
SET payload_json = json_set(
    CASE WHEN json_valid(payload_json) THEN payload_json ELSE '{}' END,
    '$.source',
    'completion_tool_miss'
)
WHERE json_extract(
    CASE WHEN json_valid(payload_json) THEN payload_json ELSE '{}' END,
    '$.source'
) = 'completion_command_miss';
