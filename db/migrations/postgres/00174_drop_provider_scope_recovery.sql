-- +goose Up
-- 桌面 Provider scope 补偿迁移已删除，其恢复账本不再有读写方。
DROP TABLE IF EXISTS provider_scope_recovery;

-- +goose Down
-- 不恢复：账本只服务于已删除的一次性补偿迁移。
