// INPUT: 当前 WorkGraph 的 Plan identity。
// OUTPUT: 该 Plan 下完整、稳定排序的 durable child Attempt 历史。
// POS: WorkGraph 专用只读历史边界；不得扩大模型与运行状态机使用的有界 Snapshot。
package orchestration

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListWorkGraphChildAttempts 返回当前 Plan 的全部 child Attempt。Snapshot 会按
// root/child lane 压缩终态以保持运行上下文有界；画布必须读取完整历史，才能把
// 每次真实 Subagent 执行投影成独立节点。
func (r *Repository) ListWorkGraphChildAttempts(
	ctx context.Context,
	planID string,
) ([]protocol.WorkAttempt, error) {
	rows, err := r.db.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
WHERE attempt.plan_id = `+r.bind(1)+`
  AND attempt.parent_attempt_id IS NOT NULL
ORDER BY attempt.work_item_id, attempt.created_at, attempt.attempt_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}
