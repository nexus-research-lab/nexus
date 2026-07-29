-- +goose Up
-- Repair audit rows left behind before Goal deletion became transactional.
DELETE FROM goal_events
WHERE NOT EXISTS (
    SELECT 1
    FROM session_goals
    WHERE session_goals.goal_id = goal_events.goal_id
);

-- +goose Down
-- Deleted orphan audit rows cannot be reconstructed safely.
SELECT 1;
