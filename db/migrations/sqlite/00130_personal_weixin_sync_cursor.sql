-- +goose Up
ALTER TABLE im_channel_accounts
ADD COLUMN sync_cursor TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE im_channel_accounts DROP COLUMN sync_cursor;
