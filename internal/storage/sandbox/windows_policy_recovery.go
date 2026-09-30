// INPUT: 精确Windows原执行双摘要及原live owner已提交的scratch完成事实。
// OUTPUT: 只收口精确关联策略，不删除文件、不猜测SDK未知原生租约。
// POS: 已有可信终态的持久对账；不是Windows原生恢复器。
package sandbox

import (
	"context"
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"time"
)

func (r *Repository) ReconcileWindowsSandboxPolicies(ctx context.Context, key protocol.WindowsSandboxKey) (int64, error) {
	p, found, err := r.WindowsSandbox(ctx, key)
	if err != nil {
		return 0, err
	}
	if !found || p.Phase != "cleaned" || p.Prepared == nil || p.Outcome == nil || !p.Outcome.Cleaned || p.Outcome.Prepared != *p.Prepared {
		return 0, ErrWindowsSandboxConflict
	}
	resources, found, err := r.WindowsSandboxResources(ctx, key.OwnerUserID, key.SessionKey, p.Intent.LeaseID)
	if err != nil {
		return 0, err
	}
	if !found || resources.Phase != "complete" || resources.ScratchRoot != p.Intent.ScratchRoot {
		return 0, ErrWindowsSandboxConflict
	}
	binding, err := json.Marshal(protocol.WindowsSandboxPolicyBinding{Key: key, Prepared: *p.Prepared})
	if err != nil {
		return 0, err
	}
	b := r.dialect.Bind
	result, err := r.db.ExecContext(ctx, `UPDATE sandbox_policy_receipts SET phase='reconciled',updated_at=`+b(1)+` WHERE owner_user_id=`+b(2)+` AND session_key=`+b(3)+` AND windows_process_json=`+b(4)+` AND lease_id=`+b(5)+` AND runtime_kind='nxs' AND phase IN ('confirmed','retiring','unknown')`, r.dialect.TimestampValue(time.Now().UTC()), key.OwnerUserID, key.SessionKey, string(binding), p.Intent.LeaseID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
