// INPUT: exact 原进程回收事实及其已完成 scratch 清理记录。
// OUTPUT: 仅关联该进程的策略回执进入 reconciled；无关联历史不变。
// POS: 策略恢复提交，不更新任务、消息、工具或其他业务结果。
package sandbox

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) ReconcileProcessPolicies(ctx context.Context, key protocol.SandboxProcessKey) (int64, error) {
	process, found, err := r.Process(ctx, key)
	if err != nil {
		return 0, err
	}
	if !found || process.Phase != protocol.SandboxProcessReaped {
		return 0, ErrProcessConflict
	}
	if process.Intent.LeaseID != "" {
		resource, found, err := r.ScratchRecovery(ctx, key.OwnerUserID, key.SessionKey, process.Intent.LeaseID)
		if err != nil {
			return 0, err
		}
		if !found || resource.Phase != protocol.SandboxScratchRecoveryComplete || resource.Scratch != process.Intent.Scratch {
			return 0, ErrScratchRecoveryConflict
		}
	}
	b := r.dialect.Bind
	var mismatches int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sandbox_policy_receipts WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND process_launch_id=`+b(3)+` AND (process_generation<>`+b(4)+` OR runtime_kind<>`+b(5)+` OR lease_id<>`+b(6)+`)`, key.OwnerUserID, key.SessionKey, key.LaunchID, key.Generation, process.Intent.RuntimeKind, process.Intent.LeaseID).Scan(&mismatches)
	if err != nil {
		return 0, err
	}
	if mismatches != 0 {
		return 0, ErrInvalidReceipt
	}
	result, err := r.db.ExecContext(ctx, `UPDATE sandbox_policy_receipts SET phase='reconciled',updated_at=`+b(1)+` WHERE owner_user_id=`+b(2)+` AND session_key=`+b(3)+` AND process_generation=`+b(4)+` AND process_launch_id=`+b(5)+` AND phase IN ('confirmed','retiring','unknown')`, r.dialect.TimestampValue(time.Now().UTC()), key.OwnerUserID, key.SessionKey, key.Generation, key.LaunchID)
	if err != nil {
		return 0, err
	}
	// Preserve the original unknown reason for audit. Reconciled phase and exact
	// process/resource records provide the new recovery fact independently.
	return result.RowsAffected()
}
