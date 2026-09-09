// INPUT: 删除协调器持有的事务、owner 与规范化 Session keys。
// OUTPUT: 本领域次级数据的事务内清理，不提交事务。
// POS: 本领域表结构与删除顺序的唯一维护入口。
package automation

import (
	"context"
	"database/sql"
	"strings"
)

// DeleteSessionRoutes 在调用方事务中清理 Session 次级引用。
func (r *Repository) DeleteSessionRoutes(ctx context.Context, tx *sql.Tx, ownerUserID string, sessionKeys []string) error {
	if strings.TrimSpace(ownerUserID) == "" || len(sessionKeys) == 0 {
		return nil
	}
	ownerBind := r.dialect.Bind(1)
	keyBinds := make([]string, 0, len(sessionKeys))
	args := make([]any, 0, len(sessionKeys)+1)
	args = append(args, ownerUserID)
	for _, sessionKey := range sessionKeys {
		args = append(args, sessionKey)
		keyBinds = append(keyBinds, r.dialect.Bind(len(args)))
	}
	inKeys := strings.Join(keyBinds, ",")
	_, err := tx.ExecContext(ctx, `DELETE FROM automation_delivery_routes
WHERE agent_id IN (SELECT id FROM agents WHERE owner_user_id = `+ownerBind+`)
 AND session_key IN (`+inKeys+`)`, args...)
	return err
}
