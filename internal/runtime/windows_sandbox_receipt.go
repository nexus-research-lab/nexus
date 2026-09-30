// INPUT: Session保留的Windows原client代次及连接后策略回执。
// OUTPUT: 原runtime launch/实际执行双摘要绑定，warm策略不改用最新probe。
// POS: 正常策略持久化与精确恢复的关联，不构造隔离能力或清理事实。
package runtime

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type windowsRuntimeProcessReader interface {
	WindowsRuntimeProcess(context.Context, string, string, uint64) (protocol.WindowsSandboxSnapshot, bool, error)
}

func bindWindowsSandboxReceiptProcess(ctx context.Context, store SandboxPolicyReceiptStore, receipt *protocol.SandboxPolicyReceiptSnapshot, generation uint64) error {
	if generation == 0 {
		return nil
	}
	reader, ok := store.(windowsRuntimeProcessReader)
	if !ok {
		return errors.New("Windows policy requires its exact runtime reader")
	}
	process, found, err := reader.WindowsRuntimeProcess(ctx, receipt.OwnerUserID, receipt.SessionKey, generation)
	if err != nil {
		return err
	}
	key := process.Intent.Key
	if !found || receipt.ProcessKey != nil || key.OwnerUserID != receipt.OwnerUserID || key.SessionKey != receipt.SessionKey || key.Generation != generation || generation > receipt.Generation || process.Intent.Purpose != "runtime" || receipt.RuntimeKind != "nxs" || process.Intent.LeaseID != receipt.LeaseID || process.Prepared == nil || (process.Phase != "started" && process.Phase != "cleaned") {
		return errors.New("Windows policy differs from its original started runtime")
	}
	receipt.WindowsProcess = &protocol.WindowsSandboxPolicyBinding{Key: key, Prepared: *process.Prepared}
	return nil
}
