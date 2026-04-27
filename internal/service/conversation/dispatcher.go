package conversation

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Dispatcher 负责按 session_key 类型路由会话请求。
type Dispatcher struct {
	dm   DMHandler
	room RoomHandler
}

// NewDispatcher 创建统一分发器。
func NewDispatcher(dm DMHandler, room RoomHandler) *Dispatcher {
	return &Dispatcher{
		dm:   dm,
		room: room,
	}
}

// HandleChat 把统一请求路由到 DM 或 Room。
func (d *Dispatcher) HandleChat(
	ctx context.Context,
	request UnifiedRequest,
) error {
	parsed := protocol.ParseSessionKey(request.SessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("room handler is not configured")
		}
		return d.room.HandleRoom(ctx, request)
	case protocol.SessionKeyKindAgent:
		if d.dm == nil {
			return errors.New("dm handler is not configured")
		}
		return d.dm.HandleDM(ctx, request)
	default:
		return protocol.StructuredSessionKeyError{
			Message: "session_key must use structured session_key format",
		}
	}
}

// HandleInterrupt 把统一中断请求路由到 DM 或 Room。
func (d *Dispatcher) HandleInterrupt(
	ctx context.Context,
	request UnifiedInterruptRequest,
) error {
	parsed := protocol.ParseSessionKey(request.SessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("room handler is not configured")
		}
		return d.room.HandleRoomInterrupt(ctx, request)
	case protocol.SessionKeyKindAgent:
		if d.dm == nil {
			return errors.New("dm handler is not configured")
		}
		return d.dm.HandleDMInterrupt(ctx, request)
	default:
		return protocol.StructuredSessionKeyError{
			Message: "session_key must use structured session_key format",
		}
	}
}
