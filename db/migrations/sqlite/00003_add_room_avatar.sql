-- +goose Up
ALTER TABLE rooms ADD COLUMN avatar VARCHAR(255);

-- +goose Down
-- SQLite 不支持直接 DROP COLUMN，回滚留空。
