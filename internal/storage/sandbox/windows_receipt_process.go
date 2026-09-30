// INPUT: 策略回执与其原Windows runtime启动代次、实际执行双摘要。
// OUTPUT: 独立Windows不可变关联，不按latest/probe或Darwin字段猜测原执行。
// POS: 持久策略审计绑定，认证和native清理由Bridge/SDK负责。
package sandbox

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// WindowsRuntimeProcess 只读取原始client代次的runtime，version probe永远不作为策略执行来源。
func (r *Repository) WindowsRuntimeProcess(ctx context.Context, owner, session string, generation uint64) (protocol.WindowsSandboxSnapshot, bool, error) {
	if r == nil || r.db == nil || owner == "" || session == "" || generation == 0 {
		return protocol.WindowsSandboxSnapshot{}, false, ErrWindowsSandboxConflict
	}
	b := r.dialect.Bind
	return r.readWindowsSandbox(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_order=2`, owner, session, generation)
}

func sameWindowsReceiptProcess(left, right *protocol.WindowsSandboxPolicyBinding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (r *Repository) validateWindowsReceiptProcess(ctx context.Context, receipt protocol.SandboxPolicyReceiptSnapshot) error {
	binding := receipt.WindowsProcess
	if binding == nil {
		return nil
	}
	key := binding.Key
	if receipt.ProcessKey != nil || key.OwnerUserID != receipt.OwnerUserID || key.SessionKey != receipt.SessionKey || key.Generation > receipt.Generation || receipt.RuntimeKind != "nxs" {
		return ErrInvalidReceipt
	}
	process, found, err := r.WindowsSandbox(ctx, key)
	if err != nil {
		return err
	}
	if !found || process.Intent.Purpose != "runtime" || process.Intent.LeaseID != receipt.LeaseID || process.Prepared == nil || *process.Prepared != binding.Prepared || (process.Phase != "started" && process.Phase != "cleaned" && process.Phase != "unknown") {
		return ErrInvalidReceipt
	}
	return nil
}
