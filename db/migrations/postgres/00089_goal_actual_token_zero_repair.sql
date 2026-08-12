-- +goose Up
-- Repair provider totals that were stored as authoritative zero despite a
-- positive token breakdown. The breakdown is the only recoverable evidence.
UPDATE goal_usage_parent_ledger
SET token_used_actual_total = GREATEST(token_used_input, 0)
        + GREATEST(token_used_cache_creation, 0)
        + GREATEST(token_used_cache_read, 0)
        + GREATEST(token_used_output, token_used_reasoning, 0),
    token_used_actual_estimated = TRUE
WHERE token_used_actual_total = 0
  AND (
      token_used_input > 0
      OR token_used_output > 0
      OR token_used_cache_creation > 0
      OR token_used_cache_read > 0
      OR token_used_reasoning > 0
  );

UPDATE session_goals
SET token_used_actual_total = GREATEST(token_used_input, 0)
        + GREATEST(token_used_cache_creation, 0)
        + GREATEST(token_used_cache_read, 0)
        + GREATEST(token_used_output, token_used_reasoning, 0),
    token_used_actual_estimated = TRUE
WHERE token_used_actual_total = 0
  AND (
      token_used_input > 0
      OR token_used_output > 0
      OR token_used_cache_creation > 0
      OR token_used_cache_read > 0
      OR token_used_reasoning > 0
  );

-- +goose Down
-- The invalid provider zero cannot be reconstructed after repair.
SELECT 1;
