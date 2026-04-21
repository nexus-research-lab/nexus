-- +goose Up
-- =====================================================
-- @File   ：00003_user_scope_isolation.sql
-- @Date   ：2026/04/21 20:55:00
-- @Author ：leemysw
-- 2026/04/21 20:55:00   Create
-- =====================================================

ALTER TABLE agents ADD COLUMN owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
ALTER TABLE agents ADD COLUMN is_main BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE rooms ADD COLUMN owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';

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
CREATE UNIQUE INDEX uq_agents_owner_main ON agents (owner_user_id, is_main) WHERE is_main = 1;
CREATE INDEX idx_agents_owner_status ON agents (owner_user_id, status, created_at);
CREATE INDEX idx_rooms_owner_created ON rooms (owner_user_id, created_at);

-- +goose Down
-- =====================================================
-- @File   ：00003_user_scope_isolation.sql
-- @Date   ：2026/04/21 20:55:00
-- @Author ：leemysw
-- 2026/04/21 20:55:00   Create
-- =====================================================

DROP INDEX IF EXISTS idx_rooms_owner_created;
DROP INDEX IF EXISTS idx_agents_owner_status;
DROP INDEX IF EXISTS uq_agents_owner_main;
DROP INDEX IF EXISTS uq_agents_owner_slug;
CREATE UNIQUE INDEX IF NOT EXISTS ix_agents_slug ON agents (slug);
