-- +goose Up
-- Source 是不可变的创建 provenance；DeliveryGrant 保存最近一次明确配置
-- 投递目标时的可信控制面/Agent 授权上下文。旧任务从 Source 精确复制一次，
-- 后续页面编辑不再破坏创建来源，也不再要求浏览器伪造 Agent actor。
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN IF NOT EXISTS delivery_grant_json TEXT NOT NULL DEFAULT '{}';

UPDATE automation_scheduled_tasks
SET delivery_grant_json = json_build_object(
    'kind', COALESCE(NULLIF(BTRIM(source_kind), ''), 'system'),
    'creator_agent_id', COALESCE(source_creator_agent_id, ''),
    'context_type', COALESCE(source_context_type, ''),
    'context_id', COALESCE(source_context_id, ''),
    'context_label', COALESCE(source_context_label, ''),
    'session_key', COALESCE(source_session_key, ''),
    'session_label', COALESCE(source_session_label, '')
)::TEXT
WHERE BTRIM(delivery_grant_json) IN ('', '{}');

-- +goose Down
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS delivery_grant_json;
