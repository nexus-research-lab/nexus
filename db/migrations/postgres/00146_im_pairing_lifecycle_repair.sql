-- +goose Up

-- Keep the route's lookup scope separate from the exact Session capability it
-- returns. Older rows used session_key only as a scope and therefore lost the
-- external IM Session key when read through the Agent-level "last" route.
ALTER TABLE automation_delivery_routes ADD COLUMN IF NOT EXISTS target_session_key VARCHAR(512) NOT NULL DEFAULT '';

-- A session-scoped row already has an exact capability in session_key. Copy it
-- first, then remove every external route that cannot prove an active pairing
-- for the exact returned Session. Legacy Agent-scoped IM rows have no such
-- proof and must not be allowed to revive after a delete/rebind.
UPDATE automation_delivery_routes
SET target_session_key = session_key
WHERE COALESCE(target_session_key, '') = ''
  AND COALESCE(session_key, '') <> ''
  AND channel IN ('discord', 'telegram', 'dingtalk', 'wechat', 'weixin-personal', 'feishu');

DELETE FROM automation_delivery_routes
WHERE channel IN ('discord', 'telegram', 'dingtalk', 'wechat', 'weixin-personal', 'feishu')
  AND NOT EXISTS (
      SELECT 1
      FROM im_pairings p
      WHERE p.agent_id = automation_delivery_routes.agent_id
        AND p.status = 'active'
        AND (
            p.session_key = automation_delivery_routes.target_session_key
            OR EXISTS (
                SELECT 1
                FROM im_pairing_sessions ps
                WHERE ps.owner_user_id = p.owner_user_id
                  AND ps.pairing_id = p.pairing_id
                  AND ps.session_key = automation_delivery_routes.target_session_key
            )
        )
  );

-- Older releases did not consistently cascade concrete projections when an
-- IM pairing was deleted. Remove rows that can no longer authorize anything.
DELETE FROM im_pairing_sessions
WHERE NOT EXISTS (
    SELECT 1
    FROM im_pairings p
    WHERE p.owner_user_id = im_pairing_sessions.owner_user_id
      AND p.pairing_id = im_pairing_sessions.pairing_id
);

-- A delivery is usable only while its pairing is active and its return address
-- is still the pairing's current direct key or an exact concrete projection.
-- Revoke stale grants instead of guessing that a recreated target is the same
-- conversation.
UPDATE im_deliveries
SET return_revoked = 1
WHERE return_revoked = 0
  AND NOT EXISTS (
      SELECT 1
      FROM im_pairings p
      WHERE p.owner_user_id = im_deliveries.owner_user_id
        AND p.pairing_id = im_deliveries.pairing_id
        AND p.status = 'active'
        AND (
            p.session_key = im_deliveries.target_session_key
            OR EXISTS (
                SELECT 1
                FROM im_pairing_sessions ps
                WHERE ps.owner_user_id = im_deliveries.owner_user_id
                  AND ps.pairing_id = p.pairing_id
                  AND ps.session_key = im_deliveries.target_session_key
            )
        )
  );

-- +goose Down

-- 只回滚新增字段；已清理的投影与撤销的授权不可恢复。
ALTER TABLE automation_delivery_routes DROP COLUMN IF EXISTS target_session_key;
