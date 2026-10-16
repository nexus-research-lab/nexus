// INPUT: 原Windows执行与精确目录身份的持久回收阶段。
// OUTPUT: 冷恢复和live Release复用同一单调仓储合同。
// POS: 不把SDK cleaned与宿主scratch complete混为同一证明。
package runtime

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type windowsScratchStore interface {
	WindowsSandboxStore
	PrepareWindowsScratchRecovery(context.Context, protocol.WindowsSandboxKey, string) (protocol.WindowsScratchRecovery, error)
	AdvanceWindowsScratchRecovery(context.Context, protocol.WindowsScratchRecovery, string) error
}

type windowsScratchLiveProof struct {
	Record  protocol.WindowsScratchRecovery
	Removed bool
}
