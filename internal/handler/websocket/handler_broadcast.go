package websocket

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BroadcastRoomEvent 广播共享 room 事件。
func (h *Handler) BroadcastRoomEvent(
	ctx context.Context,
	roomID string,
	eventType protocol.EventType,
	data map[string]any,
) {
	if h.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	event := protocol.NewEvent(eventType, data)
	event.RoomID = strings.TrimSpace(roomID)
	h.roomSubs.Broadcast(ctx, event.RoomID, event)
	h.BroadcastDirectoryChanged(ctx, string(eventType), map[string]any{
		"room_id": event.RoomID,
	})
}

// BroadcastRoomResyncRequired 广播 chat resync 通知。
func (h *Handler) BroadcastRoomResyncRequired(
	ctx context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	if h.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	data := map[string]any{
		"room_id":         strings.TrimSpace(roomID),
		"conversation_id": strings.TrimSpace(conversationID),
		"reason":          strings.TrimSpace(reason),
	}
	event := protocol.NewEvent(protocol.EventTypeRoomResyncRequired, data)
	event.RoomID = data["room_id"].(string)
	h.roomSubs.Broadcast(ctx, event.RoomID, event)
	h.BroadcastDirectoryChanged(ctx, reason, data)
}

// BroadcastDirectoryChanged 广播目录失效事件，前端收到后通过 REST 重新拉取快照。
func (h *Handler) BroadcastDirectoryChanged(ctx context.Context, reason string, data map[string]any) {
	if h.appEventSubs == nil {
		return
	}
	payload := map[string]any{}
	for key, value := range data {
		payload[key] = value
	}
	payload["reason"] = strings.TrimSpace(reason)
	h.appEventSubs.Broadcast(ctx, protocol.NewEvent(protocol.EventTypeDirectoryChanged, payload))
}

// BroadcastScheduledTaskChanged 广播定时任务失效事件，避免前端高频轮询。
func (h *Handler) BroadcastScheduledTaskChanged(ctx context.Context, event automationdomain.CronTaskEvent) {
	if h.appEventSubs == nil {
		return
	}
	message := protocol.NewEvent(protocol.EventTypeScheduledTaskChanged, map[string]any{
		"event_id": event.EventID,
		"job_id":   event.JobID,
		"agent_id": event.AgentID,
		"action":   event.Action,
		"run_id":   event.RunID,
	})
	message.AgentID = strings.TrimSpace(event.AgentID)
	h.appEventSubs.Broadcast(ctx, message)
}

// RemoveRoom 从 chat 广播注册表中移除目标 room。
func (h *Handler) RemoveRoom(roomID string) {
	if h.roomSubs == nil {
		return
	}
	h.roomSubs.RemoveRoom(roomID)
}
