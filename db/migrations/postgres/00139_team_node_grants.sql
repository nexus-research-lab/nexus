-- +goose Up
CREATE TABLE team_node_grants (
    scope TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES owner_profiles(owner_user_id),
    node_id TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('pending', 'authorized', 'revoking', 'revoked')),
    data_json TEXT NOT NULL
);
-- +goose Down
DROP TABLE team_node_grants;
