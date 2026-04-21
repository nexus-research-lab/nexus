package conversation

import (
	"context"
	"errors"

	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
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
	parsed := sessionmodel.ParseSessionKey(request.SessionKey)
	switch parsed.Kind {
	case sessionmodel.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("room handler is not configured")
		}
		return d.room.HandleRoom(ctx, request)
	case sessionmodel.SessionKeyKindAgent:
		if d.dm == nil {
			return errors.New("dm handler is not configured")
		}
		return d.dm.HandleDM(ctx, request)
	default:
		return sessionmodel.StructuredSessionKeyError{
			Message: "session_key must use structured gateway format",
		}
	}
}

// HandleInterrupt 把统一中断请求路由到 DM 或 Room。
func (d *Dispatcher) HandleInterrupt(
	ctx context.Context,
	request UnifiedInterruptRequest,
) error {
	parsed := sessionmodel.ParseSessionKey(request.SessionKey)
	switch parsed.Kind {
	case sessionmodel.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("room handler is not configured")
		}
		return d.room.HandleRoomInterrupt(ctx, request)
	case sessionmodel.SessionKeyKindAgent:
		if d.dm == nil {
			return errors.New("dm handler is not configured")
		}
		return d.dm.HandleDMInterrupt(ctx, request)
	default:
		return sessionmodel.StructuredSessionKeyError{
			Message: "session_key must use structured gateway format",
		}
	}
}
