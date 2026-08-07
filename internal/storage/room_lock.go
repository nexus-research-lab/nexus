// INPUT: 当前 SQL 事务、方言、Room ID 与可选 owner scope。
// OUTPUT: SQLite/PostgreSQL 一致的 Room-first mutation 写锁。
// POS: 跨 repository 的 Room 子资源写入锁协议；取得此锁后才能修改 member/conversation/session。
package storage

import (
	"context"
	"database/sql"
)

// LockRoomForMutation 以无语义 UPDATE 统一取得 Room 行写锁且不推进业务版本。
func LockRoomForMutation(
	ctx context.Context,
	tx *sql.Tx,
	dialect SQLDialect,
	ownerUserID string,
	roomID string,
) error {
	query := `
UPDATE rooms
SET configuration_version = configuration_version
WHERE id = ` + dialect.Bind(1)
	args := []any{roomID}
	if ownerUserID != "" {
		query += ` AND owner_user_id = ` + dialect.Bind(2)
		args = append(args, ownerUserID)
	}
	_, err := tx.ExecContext(ctx, query, args...)
	return err
}
