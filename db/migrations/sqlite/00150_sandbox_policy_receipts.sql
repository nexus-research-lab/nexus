-- +goose Up
CREATE TABLE sandbox_policy_receipts (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL,
 version INTEGER NOT NULL,
 session_id TEXT NOT NULL DEFAULT '',
 session_id_provisional BOOLEAN NOT NULL DEFAULT 0,
 runtime_kind TEXT NOT NULL,
 round_id TEXT NOT NULL DEFAULT '',
 policy_digest TEXT NOT NULL,
 required_capabilities_json TEXT NOT NULL,
 acknowledged_capabilities_json TEXT NOT NULL,
 capability_evidence TEXT NOT NULL,
 isolation_evidence TEXT NOT NULL,
 resource_policy_json TEXT NOT NULL DEFAULT '',
 lease_id TEXT NOT NULL DEFAULT '',
 phase TEXT NOT NULL,
 unknown_reason TEXT NOT NULL DEFAULT '',
 confirmed_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 PRIMARY KEY (owner_user_id, session_key, generation)
);
CREATE INDEX idx_sandbox_policy_receipts_latest
 ON sandbox_policy_receipts(owner_user_id, session_key, generation DESC);
CREATE INDEX idx_sandbox_policy_receipts_phase
 ON sandbox_policy_receipts(owner_user_id, phase, updated_at);
-- +goose Down
DROP INDEX idx_sandbox_policy_receipts_phase;
DROP INDEX idx_sandbox_policy_receipts_latest;
DROP TABLE sandbox_policy_receipts;
