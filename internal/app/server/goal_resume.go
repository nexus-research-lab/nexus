package server

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type goalContinuationDM interface {
	ShouldDeferGoalContinuation(context.Context, string, string) bool
	HandleChat(context.Context, dmsvc.Request) error
}

type goalContinuationRoom interface {
	ShouldDeferGoalContinuation(context.Context, string) bool
	DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error
}

type goalContinuationDispatcher struct {
	runtime *runtimectx.Manager
	dm      goalContinuationDM
	room    goalContinuationRoom
}

func newGoalContinuationDispatcher(runtime *runtimectx.Manager, dm *dmsvc.Service, room *roomsvc.RealtimeService) *goalContinuationDispatcher {
	return &goalContinuationDispatcher{runtime: runtime, dm: dm, room: room}
}

func (d *goalContinuationDispatcher) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if d == nil || sessionKey == "" {
		return true
	}
	if d.runtime != nil && len(d.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return true
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) == "" || d.dm == nil {
			return true
		}
		return d.dm.ShouldDeferGoalContinuation(ctx, sessionKey, parsed.AgentID)
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return true
		}
		return d.room.ShouldDeferGoalContinuation(ctx, sessionKey)
	default:
		return true
	}
}

func (d *goalContinuationDispatcher) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if d == nil {
		return errors.New("goal continuation dispatcher is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) == "" || d.dm == nil {
			return errors.New("goal continuation requires an agent session dispatcher")
		}
		return d.dm.HandleChat(ctx, dmsvc.Request{
			SessionKey:           sessionKey,
			AgentID:              parsed.AgentID,
			Content:              plan.Prompt,
			RoundID:              plan.RoundID,
			ReqID:                plan.RoundID,
			DeliveryPolicy:       protocol.ChatDeliveryPolicyQueue,
			BroadcastUserMessage: false,
			Internal:             true,
			InputOptions: sdkprotocol.OutboundMessageOptions{
				Synthetic:      plan.Synthetic,
				HiddenFromUser: plan.HiddenFromUser,
				Purpose:        plan.Purpose,
				Priority:       "internal",
				Metadata:       plan.Metadata,
			},
		})
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("goal continuation requires a room session dispatcher")
		}
		return d.room.DispatchGoalContinuation(ctx, plan)
	default:
		return errors.New("goal continuation only supports agent or room session keys")
	}
}
