-- +goose Up
-- SQLite legacy version collisions are repaired transactionally before Goose
-- runs because SQLite cannot conditionally add an existing column.
SELECT 1;

-- +goose Down
SELECT 1;
