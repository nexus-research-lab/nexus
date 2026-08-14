package server

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

type goalInterruptDM interface {
	HandleInterrupt(context.Context, dmsvc.InterruptRequest) error
}

type goalInterruptRoom interface {
	HandleInterrupt(context.Context, roomrealtime.InterruptRequest) error
}

type goalInterruptDispatcher struct {
	dm   goalInterruptDM
	room goalInterruptRoom
}

func newGoalInterruptDispatcher(dm *dmsvc.Service, room *roomrealtime.Service) *goalInterruptDispatcher {
	return &goalInterruptDispatcher{dm: dm, room: room}
}

func (d *goalInterruptDispatcher) InterruptGoalRuntime(ctx context.Context, sessionKey string, roundIDs []string) error {
	if d == nil {
		return errors.New("goal runtime interrupter is not configured")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	normalizedRoundIDs := make([]string, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		if roundID = strings.TrimSpace(roundID); roundID != "" {
			normalizedRoundIDs = append(normalizedRoundIDs, roundID)
		}
	}
	if len(normalizedRoundIDs) == 0 {
		// Goal pause is allowed to persist without a live runtime. Never widen a
		// missing exact accounting target into a session-wide interrupt.
		return nil
	}
	var interruptErrors []error
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return nil
		}
		for _, roundID := range normalizedRoundIDs {
			if err := d.room.HandleInterrupt(ctx, roomrealtime.InterruptRequest{SessionKey: sessionKey, AgentRoundID: roundID}); err != nil {
				interruptErrors = append(interruptErrors, err)
			}
		}
		return errors.Join(interruptErrors...)
	case protocol.SessionKeyKindAgent:
		if parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" {
			if d.room == nil {
				return nil
			}
			for _, roundID := range normalizedRoundIDs {
				if err := d.room.HandleInterrupt(ctx, roomrealtime.InterruptRequest{
					SessionKey:   protocol.BuildRoomSharedSessionKey(parsed.Ref),
					AgentRoundID: roundID,
				}); err != nil {
					interruptErrors = append(interruptErrors, err)
				}
			}
			return errors.Join(interruptErrors...)
		}
		if d.dm == nil {
			return nil
		}
		for _, roundID := range normalizedRoundIDs {
			if err := d.dm.HandleInterrupt(ctx, dmsvc.InterruptRequest{SessionKey: sessionKey, RoundID: roundID}); err != nil {
				interruptErrors = append(interruptErrors, err)
			}
		}
		return errors.Join(interruptErrors...)
	default:
		return nil
	}
}
