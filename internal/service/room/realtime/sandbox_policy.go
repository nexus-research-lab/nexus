// INPUT: Exact Room slot whose host-managed sandbox boundary has changed.
// OUTPUT: Cancelled old work and approvals, followed by confirmed runtime cleanup.
// POS: Sandbox mode changes stop the old attempt; no prompt or tool is replayed.
package realtime

import (
	"context"
	"fmt"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *Service) closeSlotForSandboxPolicyChange(ctx context.Context, slot *activeRoomSlot, client runtimectx.Client) error {
	const reason = "沙箱权限已变更，当前执行已停止；新请求将使用更新后的权限"
	slot.suppressOutput()
	slot.setInterruptReason(reason)
	slot.setStatus("cancelled")
	slot.cancelRuntime()
	if s.permission != nil {
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, reason)
	}
	client.Retire()
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	if err := client.Disconnect(closeCtx); err != nil {
		return fmt.Errorf("等待旧 Room sandbox runtime 退出: %w", err)
	}
	return nil
}
