// INPUT: owner-scoped 创建 request_id。
// OUTPUT: 创建 ledger 是否存在及其仍存活的任务快照。
// POS: Automation 创建幂等回执的只读仓储；不暴露 intent digest。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetScheduledTaskCreateRequestResult 返回 request_id 是否曾被原子认领，
// 以及其对应任务是否仍存在。
func (r *Repository) GetScheduledTaskCreateRequestResult(
	ctx context.Context,
	ownerUserID string,
	requestID string,
) (*automationdomain.ScheduledTask, bool, error) {
	var jobID string
	err := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`SELECT job_id
FROM automation_task_create_requests
WHERE owner_user_id = %s AND request_id = %s`,
			r.bind(1),
			r.bind(2),
		),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(requestID),
	).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	task, err := r.GetScheduledTask(ctx, strings.TrimSpace(ownerUserID), strings.TrimSpace(jobID))
	if err != nil {
		return nil, true, err
	}
	return task, true, nil
}
