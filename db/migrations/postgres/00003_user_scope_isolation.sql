-- +goose Up
-- =====================================================
-- @File   ：00003_user_scope_isolation.sql
-- @Date   ：2026/04/21 20:55:00
-- @Author ：leemysw
-- 2026/04/21 20:55:00   Create
-- =====================================================

ALTER TABLE agents ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_main BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';

UPDATE agents
SET owner_user_id = COALESCE(
    (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
    (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
    '__system__'
)
WHERE owner_user_id = '__system__';

UPDATE rooms
SET owner_user_id = COALESCE(
    (SELECT user_id FROM users WHERE role = 'owner' AND status = 'active' ORDER BY created_at ASC LIMIT 1),
    (SELECT user_id FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1),
    '__system__'
)
WHERE owner_user_id = '__system__';

DROP INDEX IF EXISTS ix_agents_slug;
CREATE UNIQUE INDEX uq_agents_owner_slug ON agents (owner_user_id, slug);
CREATE UNIQUE INDEX uq_agents_owner_main ON agents (owner_user_id) WHERE is_main = TRUE;
CREATE INDEX idx_agents_owner_status ON agents (owner_user_id, status, created_at);
CREATE INDEX idx_rooms_owner_created ON rooms (owner_user_id, created_at);

ALTER TABLE agents ALTER COLUMN owner_user_id DROP DEFAULT;
ALTER TABLE agents ALTER COLUMN is_main DROP DEFAULT;
ALTER TABLE rooms ALTER COLUMN owner_user_id DROP DEFAULT;

-- +goose Down

ALTER TABLE agents ALTER COLUMN owner_user_id SET DEFAULT '__system__';
ALTER TABLE agents ALTER COLUMN is_main SET DEFAULT FALSE;
ALTER TABLE rooms ALTER COLUMN owner_user_id SET DEFAULT '__system__';

DROP INDEX IF EXISTS idx_rooms_owner_created;
DROP INDEX IF EXISTS idx_agents_owner_status;
DROP INDEX IF EXISTS uq_agents_owner_main;
DROP INDEX IF EXISTS uq_agents_owner_slug;
CREATE UNIQUE INDEX ix_agents_slug ON agents (slug);

ALTER TABLE rooms DROP COLUMN owner_user_id;
ALTER TABLE agents DROP COLUMN is_main;
ALTER TABLE agents DROP COLUMN owner_user_id;
