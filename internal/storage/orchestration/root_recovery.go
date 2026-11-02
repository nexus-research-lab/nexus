// INPUT: A startup scan cursor over current managed Executions.
// OUTPUT: Bounded authoritative Execution batches for root-attempt recovery.
// POS: SQL ownership for process-restart reconciliation; no runtime decisions.
package orchestration

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListRecoveryExecutions returns active managed executions in stable ID order.
func (r *Repository) ListRecoveryExecutions(ctx context.Context, after string, limit int) ([]protocol.Execution, error) {
	rows, err := r.db.QueryContext(ctx, r.executionSelect()+`
 WHERE executions.status = 'active' AND executions.execution_id > `+r.bind(1)+`
 AND EXISTS (SELECT 1 FROM execution_plan_revisions p WHERE p.execution_id=executions.execution_id AND p.status='active')
 ORDER BY executions.execution_id LIMIT `+r.bind(2), after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []protocol.Execution
	for rows.Next() {
		value, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
