-- +goose Up
-- =====================================================
-- @File   ：00004_drop_alembic_version.sql
-- @Date   ：2026/04/21 18:12:00
-- @Author ：leemysw
-- 2026/04/21 18:12:00   Create
-- =====================================================

DROP TABLE IF EXISTS alembic_version;

-- +goose Down
-- =====================================================
-- @File   ：00004_drop_alembic_version.sql
-- @Date   ：2026/04/21 18:12:00
-- @Author ：leemysw
-- 2026/04/21 18:12:00   Create
-- =====================================================

CREATE TABLE IF NOT EXISTS alembic_version (
    version_num VARCHAR(32) NOT NULL PRIMARY KEY
);
