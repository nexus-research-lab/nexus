// INPUT: owner/session 与有界历史目录请求。
// OUTPUT: Execution 元信息和当前 Plan 节点数；不读取 Snapshot、Runtime Graph 或 transcript。
// POS: WorkGraph 历史目录和草图来源目录共用的摘要查询。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListManagedSummaries 用两个有界查询读取目录与节点数，避免逐图构建完整视图。
func (r *Repository) ListManagedSummaries(ctx context.Context, owner, session string, limit int) ([]protocol.ExecutionSummary, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	executions, err := r.listManaged(ctx, tx, owner, session, limit)
	if err != nil {
		return nil, err
	}
	result := make([]protocol.ExecutionSummary, 0, len(executions))
	if len(executions) == 0 {
		return result, nil
	}
	args := make([]any, len(executions))
	binds := make([]string, len(executions))
	for i, execution := range executions {
		args[i], binds[i] = execution.ID, r.bind(i+1)
	}
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
SELECT plan.execution_id, COUNT(item.work_item_id)
FROM execution_plan_revisions plan
JOIN execution_plan_items item ON item.plan_id = plan.plan_id AND item.execution_id = plan.execution_id
WHERE plan.status = 'active' AND plan.execution_id IN (%s)
GROUP BY plan.execution_id`, strings.Join(binds, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int, len(executions))
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, e := range executions {
		result = append(result, protocol.ExecutionSummary{
			ID: e.ID, SessionKey: e.SessionKey, Objective: e.Objective, Status: e.Status,
			Version: e.Version, WorkItemCount: counts[e.ID], CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		})
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return result, tx.Commit()
}
