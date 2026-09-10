// INPUT: 删除协调器持有的事务、owner 与规范化 Session keys。
// OUTPUT: 本领域次级数据的事务内清理，不提交事务。
// POS: 本领域表结构与删除顺序的唯一维护入口。
package orchestration

import (
	"context"
	"database/sql"
	"strings"
)

// DeleteSessionReferences 在调用方事务中清理 Session 次级引用。
func (r *Repository) DeleteSessionReferences(ctx context.Context, tx *sql.Tx, ownerUserID string, sessionKeys []string) error {
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
	statements := []string{
		`DELETE FROM runtime_graph_artifact_refs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM runtime_graph_edge_runs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM runtime_graph_node_runs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM execution_plan_proposals
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM executions
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return err
		}
	}
	return nil
}
