-- +goose Up

-- prepare 原子选择的 durable pointer。room/conversation 使用空串规范化，确保
-- 一个 exact owner/session/scope/coordinator 只能拥有一个 active proposal。
CREATE TABLE execution_plan_proposal_bindings (
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    scope_kind VARCHAR(16) NOT NULL,
    room_id VARCHAR(64) NOT NULL DEFAULT '',
    conversation_id VARCHAR(64) NOT NULL DEFAULT '',
    coordinator_agent_id VARCHAR(128) NOT NULL,
    proposal_id VARCHAR(64) NOT NULL,
    root_round_id VARCHAR(128) NOT NULL,
    runtime_round_id VARCHAR(128) NOT NULL DEFAULT '',
    agent_round_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (
        owner_user_id, session_key, scope_kind, room_id,
        conversation_id, coordinator_agent_id
    ),
    CONSTRAINT uq_execution_plan_proposal_bindings_proposal UNIQUE (proposal_id),
    CONSTRAINT ck_execution_plan_proposal_bindings_scope_kind
        CHECK (scope_kind IN ('dm', 'room')),
    CONSTRAINT ck_execution_plan_proposal_bindings_scope_identity
        CHECK (
            (scope_kind = 'dm' AND room_id = '' AND conversation_id = '')
            OR
            (scope_kind = 'room' AND room_id <> '' AND conversation_id <> '')
        ),
    CONSTRAINT ck_execution_plan_proposal_bindings_root_round
        CHECK (length(trim(root_round_id)) > 0),
    FOREIGN KEY (proposal_id)
        REFERENCES execution_plan_proposals(proposal_id) ON DELETE CASCADE
);

-- 旧数据只在 exact scope 中恰有一个 sealed candidate 时安全承接；多候选保持
-- unbound，下一次显式 prepare 会原子选中新 proposal 并 discard 旧 candidates。
INSERT INTO execution_plan_proposal_bindings (
    owner_user_id, session_key, scope_kind, room_id, conversation_id,
    coordinator_agent_id, proposal_id,
    root_round_id, runtime_round_id, agent_round_id,
    created_at, updated_at
)
SELECT
    candidate.owner_user_id,
    candidate.session_key,
    candidate.scope_kind,
    COALESCE(candidate.room_id, ''),
    COALESCE(candidate.conversation_id, ''),
    candidate.coordinator_agent_id,
    candidate.proposal_id,
    candidate.root_round_id,
    COALESCE(candidate.runtime_round_id, ''),
    COALESCE(candidate.agent_round_id, ''),
    candidate.created_at,
    candidate.updated_at
FROM execution_plan_proposals AS candidate
WHERE candidate.status = 'sealed'
  AND NOT EXISTS (
      SELECT 1
      FROM execution_plan_proposals AS competing
      WHERE competing.owner_user_id = candidate.owner_user_id
        AND competing.session_key = candidate.session_key
        AND competing.scope_kind = candidate.scope_kind
        AND COALESCE(competing.room_id, '') = COALESCE(candidate.room_id, '')
        AND COALESCE(competing.conversation_id, '') = COALESCE(candidate.conversation_id, '')
        AND competing.coordinator_agent_id = candidate.coordinator_agent_id
        AND competing.status = 'sealed'
        AND competing.proposal_id <> candidate.proposal_id
  );

-- +goose Down

DROP TABLE IF EXISTS execution_plan_proposal_bindings;
