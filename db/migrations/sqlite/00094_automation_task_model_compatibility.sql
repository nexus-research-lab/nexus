-- +goose Up
-- 历史版本可能把结构化会话键放在 delivery_to，或只留在 source_session_key。
-- 只映射能够精确识别的 session_key；任务内容、调度、启停与权限策略均不改动。
UPDATE automation_scheduled_tasks
SET delivery_session_key = TRIM(delivery_to)
WHERE COALESCE(TRIM(delivery_session_key), '') = ''
  AND delivery_mode = 'explicit'
  AND (
      TRIM(delivery_to) LIKE 'agent:%'
      OR TRIM(delivery_to) LIKE 'room:group:%'
  );

UPDATE automation_scheduled_tasks
SET delivery_session_key = TRIM(source_session_key)
WHERE COALESCE(TRIM(delivery_session_key), '') = ''
  AND delivery_mode = 'last'
  AND (
      TRIM(source_session_key) LIKE 'agent:%'
      OR TRIM(source_session_key) LIKE 'room:group:%'
  );

-- 旧 UI 以 explicit + websocket 包装外部 IM session。新模型直接保存逻辑
-- session route，运行和投递前再校验 active pairing，避免把旧账号误投递为 Web DM。
UPDATE automation_scheduled_tasks
SET delivery_mode = 'last',
    delivery_channel = NULL,
    delivery_to = NULL,
    delivery_account_id = NULL,
    delivery_thread_id = NULL
WHERE delivery_mode = 'explicit'
  AND TRIM(delivery_to) = TRIM(delivery_session_key)
  AND COALESCE(LOWER(TRIM(delivery_channel)), '') IN ('', 'ws', 'websocket', 'internal')
  AND (
      delivery_session_key LIKE 'agent:%:dg:%'
      OR delivery_session_key LIKE 'agent:%:discord:%'
      OR delivery_session_key LIKE 'agent:%:tg:%'
      OR delivery_session_key LIKE 'agent:%:telegram:%'
      OR delivery_session_key LIKE 'agent:%:dt:%'
      OR delivery_session_key LIKE 'agent:%:dingtalk:%'
      OR delivery_session_key LIKE 'agent:%:wx:%'
      OR delivery_session_key LIKE 'agent:%:wechat:%'
      OR delivery_session_key LIKE 'agent:%:weixin-personal:%'
      OR delivery_session_key LIKE 'agent:%:fs:%'
      OR delivery_session_key LIKE 'agent:%:feishu:%'
  );

-- +goose Down
-- 数据归一化不删除用户数据，也无法可靠区分迁移后的旧任务和升级后新建任务；
-- 因而回滚版本标记时保持规范化结果，避免把新任务错误降级。
SELECT 1;
