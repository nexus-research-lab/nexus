-- +goose Up
-- A legacy deployment may have recorded version 56 for conversation drafts
-- before disabled Skill storage took that version.
ALTER TABLE runtimes
ADD COLUMN IF NOT EXISTS disabled_skill_ids_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
-- Version 56 owns this column for current installations.
SELECT 1;
