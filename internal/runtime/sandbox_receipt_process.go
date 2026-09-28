// INPUT: Manager 保留的 client 原始启动代次与连接后的策略回执。
// OUTPUT: 当前策略代次绑定到原 runtime launch；缺失或错误身份即拒绝。
// POS: 策略/进程恢复关联，不从最新记录、探测或 scratch 路径推断身份。
package runtime

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sandboxRuntimeProcessReader interface {
	RuntimeProcess(context.Context, string, string, uint64) (protocol.SandboxProcessSnapshot, bool, error)
}

func bindSandboxReceiptProcess(ctx context.Context, store SandboxPolicyReceiptStore, receipt *protocol.SandboxPolicyReceiptSnapshot, generation uint64) error {
	if generation == 0 {
		return nil
	}
	reader, ok := store.(sandboxRuntimeProcessReader)
	if !ok {
		return errors.New("supervised policy requires exact runtime process reader")
	}
	process, found, err := reader.RuntimeProcess(ctx, receipt.OwnerUserID, receipt.SessionKey, generation)
	if err != nil {
		return err
	}
	key := process.Intent.Key
	if !found || key.OwnerUserID != receipt.OwnerUserID || key.SessionKey != receipt.SessionKey || key.Generation != generation || generation > receipt.Generation || !validSandboxLaunchID(key.LaunchID) || (process.Intent.Purpose != protocol.SandboxProcessRuntime && process.Intent.Purpose != "") || process.Intent.RuntimeKind != receipt.RuntimeKind || process.Intent.LeaseID != receipt.LeaseID || (process.Phase != protocol.SandboxProcessReleased && process.Phase != protocol.SandboxProcessReaped) {
		return errors.New("supervised policy does not match its original released runtime")
	}
	receipt.ProcessKey = &key
	return nil
}
