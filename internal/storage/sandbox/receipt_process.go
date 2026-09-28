// INPUT: 已确认策略及原监督 runtime 的精确 owner/session/generation/launch 身份。
// OUTPUT: 持久策略到原进程的不可变关联；旧回执不补猜关联。
// POS: 崩溃恢复的身份事实，不自动清除策略、资源或业务 unknown。
package sandbox

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RuntimeProcess selects the runtime at an exact original client generation,
// never the latest launch or a version/admission probe.
func (r *Repository) RuntimeProcess(ctx context.Context, owner, session string, generation uint64) (protocol.SandboxProcessSnapshot, bool, error) {
	if r == nil || r.db == nil || owner == "" || session == "" || generation == 0 {
		return protocol.SandboxProcessSnapshot{}, false, ErrInvalidProcess
	}
	b := r.dialect.Bind
	return r.readProcess(ctx, `WHERE owner_user_id=`+b(1)+` AND session_key=`+b(2)+` AND generation=`+b(3)+` AND launch_order=4`, owner, session, generation)
}

func sameReceiptProcess(a, b *protocol.SandboxProcessKey) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (r *Repository) validateReceiptProcess(ctx context.Context, receipt protocol.SandboxPolicyReceiptSnapshot) error {
	key := receipt.ProcessKey
	if key == nil {
		return nil
	}
	if key.OwnerUserID != receipt.OwnerUserID || key.SessionKey != receipt.SessionKey || key.Generation > receipt.Generation {
		return fmt.Errorf("%w: process scope mismatch", ErrInvalidReceipt)
	}
	process, found, err := r.Process(ctx, *key)
	if err != nil {
		return err
	}
	if !found || (process.Intent.Purpose != protocol.SandboxProcessRuntime && process.Intent.Purpose != "") || process.Intent.RuntimeKind != receipt.RuntimeKind || process.Intent.LeaseID != receipt.LeaseID || (process.Phase != protocol.SandboxProcessReleased && process.Phase != protocol.SandboxProcessReaped) {
		return fmt.Errorf("%w: policy requires its released runtime and exact lease", ErrInvalidReceipt)
	}
	return nil
}
