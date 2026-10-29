-- +goose Up
-- macOS 桌面沙箱改为每条命令独立 Seatbelt，宿主不再持有启动记录、策略回执或 scratch 恢复状态；这些表已无读取方。
DROP TABLE sandbox_scratch_recoveries;
DROP TABLE sandbox_process_launches;
DROP TABLE sandbox_policy_receipts;

-- +goose Down
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
 process_generation BIGINT NOT NULL DEFAULT 0 CHECK (process_generation >= 0 AND process_generation <= generation),
 process_launch_id TEXT NOT NULL DEFAULT '',
 PRIMARY KEY (owner_user_id, session_key, generation)
);
CREATE INDEX idx_sandbox_policy_receipts_latest ON sandbox_policy_receipts(owner_user_id, session_key, generation DESC);
CREATE INDEX idx_sandbox_policy_receipts_phase ON sandbox_policy_receipts(owner_user_id, phase, updated_at);
CREATE INDEX idx_sandbox_policy_process ON sandbox_policy_receipts(owner_user_id, session_key, process_launch_id) WHERE process_launch_id <> '';
CREATE INDEX idx_sandbox_policy_recovery ON sandbox_policy_receipts(process_launch_id) WHERE process_launch_id<>'' AND phase IN ('confirmed','retiring','unknown');

CREATE TABLE sandbox_process_launches (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 generation BIGINT NOT NULL CHECK (generation > 0),
 launch_order INTEGER NOT NULL DEFAULT 4 CHECK (launch_order BETWEEN 1 AND 4),
 launch_id TEXT NOT NULL UNIQUE,
 phase TEXT NOT NULL CHECK (phase IN ('prepared', 'registered', 'released', 'aborted', 'reaped')),
 intent_json TEXT NOT NULL,
 registration_json TEXT NOT NULL DEFAULT '',
 evidence_json TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMP NOT NULL,
 updated_at TIMESTAMP NOT NULL,
 resource_phase TEXT NOT NULL DEFAULT 'pending' CHECK (resource_phase IN ('pending','complete')),
 PRIMARY KEY (owner_user_id, session_key, generation, launch_order)
);
CREATE UNIQUE INDEX idx_sandbox_process_active ON sandbox_process_launches(owner_user_id,session_key) WHERE phase IN ('prepared','registered','released');
CREATE INDEX idx_sandbox_process_latest ON sandbox_process_launches(owner_user_id,session_key,generation DESC,launch_order DESC);
CREATE INDEX idx_sandbox_process_pending_launch ON sandbox_process_launches(launch_id) WHERE phase IN ('prepared', 'registered', 'released');
CREATE INDEX idx_sandbox_resource_recovery ON sandbox_process_launches(launch_id) WHERE resource_phase='pending';

CREATE TABLE sandbox_scratch_recoveries (
 owner_user_id TEXT NOT NULL,
 session_key TEXT NOT NULL,
 lease_id TEXT NOT NULL,
 source_generation BIGINT NOT NULL CHECK (source_generation > 0),
 source_launch_id TEXT NOT NULL,
 scratch_json TEXT NOT NULL,
 phase TEXT NOT NULL CHECK (phase IN ('prepared','quarantined','deleting','complete')),
 PRIMARY KEY (owner_user_id,session_key,lease_id)
);
CREATE INDEX idx_sandbox_scratch_pending ON sandbox_scratch_recoveries(owner_user_id,session_key) WHERE phase <> 'complete';
