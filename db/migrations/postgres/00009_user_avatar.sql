-- +goose Up
-- =====================================================
-- @File   ：00009_user_avatar.sql
-- @Date   ：2026/04/25 16:18:00
-- @Author ：leemysw
-- 2026/04/25 16:18:00   Create
-- =====================================================

ALTER TABLE users ADD COLUMN avatar VARCHAR(255);

-- +goose Down
-- =====================================================
-- @File   ：00009_user_avatar.sql
-- @Date   ：2026/04/25 16:18:00
-- @Author ：leemysw
-- 2026/04/25 16:18:00   Create
-- =====================================================

ALTER TABLE users DROP COLUMN avatar;
