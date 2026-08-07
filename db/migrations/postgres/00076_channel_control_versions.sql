-- +goose Up
-- Channel control publication and revocation generation.
CREATE TABLE channel_control_versions (
    owner_user_id VARCHAR(64) NOT NULL PRIMARY KEY,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO channel_control_versions (owner_user_id, version)
SELECT owner_user_id, 1 FROM im_channel_configs
UNION
SELECT owner_user_id, 1 FROM im_channel_accounts
UNION
SELECT owner_user_id, 1 FROM im_pairings
ON CONFLICT (owner_user_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS channel_control_versions;
