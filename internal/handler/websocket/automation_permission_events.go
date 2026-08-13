// INPUT: Automation 服务生成的 DM/Room permission request/resolved 事件。
// OUTPUT: 当前 Session 绑定与 Room 全局订阅共享的交互状态广播。
// POS: Automation 持久权限到 WebSocket transport 的薄适配层；不解释审批决策。
package websocket

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// NotifyAutomationPermissionEvent 实现 Automation 的 Session 事件通知出口。
func (h *Handler) NotifyAutomationPermissionEvent(
	ctx context.Context,
	event protocol.EventMessage,
) {
	if h == nil {
		return
	}
	if h.permission != nil && strings.TrimSpace(event.SessionKey) != "" {
		_ = h.permission.BroadcastEvent(ctx, event.SessionKey, event)
	}
	if h.roomSubs != nil && strings.TrimSpace(event.RoomID) != "" {
		roomEvent := event
		roomEvent.DeliveryMode = protocol.DeliveryModeDurable
		_ = h.roomSubs.Broadcast(ctx, event.RoomID, roomEvent)
	}
}
