-- +goose Up
CREATE TABLE team_node_jobs (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    owner_user_id TEXT NOT NULL REFERENCES owner_profiles(owner_user_id),
    local_agent_id TEXT NOT NULL,
    state TEXT NOT NULL,
    scope TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    data_json TEXT NOT NULL
);
CREATE UNIQUE INDEX team_node_jobs_active_agent ON team_node_jobs(owner_user_id, local_agent_id)
WHERE state IN ('claiming','ready','running','draining','review_required');
CREATE INDEX team_node_jobs_node ON team_node_jobs(node_id, state);
CREATE INDEX team_node_jobs_scope ON team_node_jobs(owner_user_id, scope, created_at);
CREATE TABLE team_node_outputs (
    job_id TEXT NOT NULL REFERENCES team_node_jobs(id),
    sequence INTEGER NOT NULL,
    output_id TEXT NOT NULL,
    data_json TEXT NOT NULL,
    sent BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY(job_id, sequence)
);
-- +goose Down
DROP TABLE team_node_outputs;
DROP TABLE team_node_jobs;
