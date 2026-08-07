-- +goose Up
-- Channel control publication and revocation generation.
CREATE TABLE channel_control_versions (
    owner_user_id VARCHAR(64) NOT NULL PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO channel_control_versions (owner_user_id, version)
SELECT owner_user_id, 1 FROM im_channel_configs
UNION
SELECT owner_user_id, 1 FROM im_channel_accounts
UNION
SELECT owner_user_id, 1 FROM im_pairings;

-- +goose Down
DROP TABLE IF EXISTS channel_control_versions;
