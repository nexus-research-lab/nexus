package server

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type goalInterruptDM interface {
	HandleInterrupt(context.Context, dmsvc.InterruptRequest) error
}

type goalInterruptRoom interface {
	HandleInterrupt(context.Context, roomsvc.InterruptRequest) error
}

type goalInterruptDispatcher struct {
	dm   goalInterruptDM
	room goalInterruptRoom
}

func newGoalInterruptDispatcher(dm *dmsvc.Service, room *roomsvc.RealtimeService) *goalInterruptDispatcher {
	return &goalInterruptDispatcher{dm: dm, room: room}
}

func (d *goalInterruptDispatcher) InterruptGoalRuntime(ctx context.Context, sessionKey string) error {
	if d == nil {
		return errors.New("goal runtime interrupter is not configured")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return nil
		}
		return d.room.HandleInterrupt(ctx, roomsvc.InterruptRequest{SessionKey: sessionKey})
	case protocol.SessionKeyKindAgent:
		if parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" {
			if d.room == nil {
				return nil
			}
			return d.room.HandleInterrupt(ctx, roomsvc.InterruptRequest{
				SessionKey: protocol.BuildRoomSharedSessionKey(parsed.Ref),
			})
		}
		if d.dm == nil {
			return nil
		}
		return d.dm.HandleInterrupt(ctx, dmsvc.InterruptRequest{SessionKey: sessionKey})
	default:
		return nil
	}
}
