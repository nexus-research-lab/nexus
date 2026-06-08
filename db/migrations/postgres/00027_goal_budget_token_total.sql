-- +goose Up

UPDATE session_goals
SET token_used_total = GREATEST(token_used_input, 0) + GREATEST(token_used_output, 0);

-- +goose Down

UPDATE session_goals
SET token_used_total = token_used_input
    + token_used_output
    + token_used_cache_creation
    + token_used_cache_read
    + token_used_reasoning;
