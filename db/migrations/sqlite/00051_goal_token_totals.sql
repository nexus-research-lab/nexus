-- +goose Up
ALTER TABLE session_goals
ADD COLUMN token_used_actual_total INTEGER NOT NULL DEFAULT 0;

ALTER TABLE session_goals
ADD COLUMN token_used_actual_estimated BOOLEAN NOT NULL DEFAULT 0;

UPDATE session_goals
SET token_used_actual_total = CASE
        WHEN token_used_input != 0
          OR token_used_output != 0
          OR token_used_cache_creation != 0
          OR token_used_cache_read != 0
          OR token_used_reasoning != 0
        THEN MAX(token_used_input, 0)
           + MAX(token_used_cache_creation, 0)
           + MAX(token_used_cache_read, 0)
           + MAX(token_used_output, token_used_reasoning, 0)
        ELSE MAX(token_used_total, 0)
    END,
    token_used_actual_estimated = CASE
        WHEN token_used_input != 0
          OR token_used_output != 0
          OR token_used_cache_creation != 0
          OR token_used_cache_read != 0
          OR token_used_reasoning != 0
          OR token_used_total != 0
        THEN 1
        ELSE 0
    END,
    token_used_total = CASE
        WHEN token_used_input != 0
          OR token_used_output != 0
          OR token_used_cache_creation != 0
          OR token_used_cache_read != 0
          OR token_used_reasoning != 0
        THEN MAX(token_used_input, 0) + MAX(token_used_output, 0)
        ELSE MAX(token_used_total, 0)
    END;

-- +goose Down
ALTER TABLE session_goals DROP COLUMN token_used_actual_estimated;
ALTER TABLE session_goals DROP COLUMN token_used_actual_total;
