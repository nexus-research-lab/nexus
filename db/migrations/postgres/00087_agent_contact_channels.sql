-- +goose Up
ALTER TABLE rooms ADD COLUMN is_contact_channel BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE contacts
    ADD COLUMN direct_room_id VARCHAR(64) REFERENCES rooms (id) ON DELETE SET NULL;
CREATE INDEX idx_contacts_direct_room ON contacts (direct_room_id);

-- +goose Down
DROP INDEX IF EXISTS idx_contacts_direct_room;
ALTER TABLE contacts DROP COLUMN direct_room_id;
ALTER TABLE rooms DROP COLUMN is_contact_channel;
