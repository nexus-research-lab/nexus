-- +goose Up
DROP INDEX uq_execution_acceptances_work_spec;
CREATE TABLE execution_submission_supersessions (
 submission_id TEXT PRIMARY KEY REFERENCES execution_submissions(submission_id) ON DELETE CASCADE,
 command_id TEXT NOT NULL,
 created_at DATETIME NOT NULL
);
-- +goose Down
-- Refuse downgrade when retry history contains multiple accepted generations.
CREATE UNIQUE INDEX uq_execution_acceptances_work_spec ON execution_acceptances(work_item_id,spec_id) WHERE decision='accepted';
DROP TABLE execution_submission_supersessions;
