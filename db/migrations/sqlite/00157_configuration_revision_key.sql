-- +goose Up
-- Host-only revision metadata. Version 0 is the one-time bootstrap slot;
-- application code installs a cryptographically random key with a CAS.
CREATE TABLE IF NOT EXISTS configuration_revision_key (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    version INTEGER NOT NULL,
    key_hex TEXT NOT NULL
);
INSERT INTO configuration_revision_key (singleton, version, key_hex)
SELECT 1, 0, ''
WHERE NOT EXISTS (SELECT 1 FROM configuration_revision_key WHERE singleton = 1);

-- +goose Down
DROP TABLE configuration_revision_key;
