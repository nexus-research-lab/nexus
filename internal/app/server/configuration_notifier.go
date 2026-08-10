// INPUT: configuration 写入后的 Agent/Room 失效通知与 websocket broadcaster。
// OUTPUT: 目录刷新、Room resync 与成员变更事件。
// POS: configuration 领域事件到实时 UI 投影的应用层适配器。
package server

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/handler/websocket"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type configurationRealtimeNotifier struct {
	broadcaster *websocket.Handler
}

func (n configurationRealtimeNotifier) AgentChanged(ctx context.Context, agentID, reason string) {
	n.broadcaster.BroadcastDirectoryChanged(ctx, reason, map[string]any{"agent_id": agentID})
}

func (n configurationRealtimeNotifier) RoomChanged(
	ctx context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	n.broadcaster.BroadcastRoomResyncRequired(ctx, roomID, conversationID, reason)
}

func (n configurationRealtimeNotifier) RoomMemberChanged(
	ctx context.Context,
	roomID string,
	agentID string,
	added bool,
) {
	eventType := protocol.EventTypeRoomMemberRemoved
	if added {
		eventType = protocol.EventTypeRoomMemberAdded
	}
	n.broadcaster.BroadcastRoomEvent(ctx, roomID, eventType, map[string]any{
		"room_id": roomID, "agent_id": agentID,
	})
}
