package websocket

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

type goalEventBroadcaster struct {
	nexusBroadcaster interface {
		BroadcastEvent(context.Context, string, protocol.EventMessage) []error
	}
	rpcSubscribers *appServerGoalRPCRegistry
}

func newGoalEventBroadcaster(
	nexusBroadcaster interface {
		BroadcastEvent(context.Context, string, protocol.EventMessage) []error
	},
	rpcSubscribers *appServerGoalRPCRegistry,
) *goalEventBroadcaster {
	return &goalEventBroadcaster{
		nexusBroadcaster: nexusBroadcaster,
		rpcSubscribers:   rpcSubscribers,
	}
}

func (b *goalEventBroadcaster) BroadcastEvent(ctx context.Context, sessionKey string, event protocol.EventMessage) []error {
	errs := []error(nil)
	if b.nexusBroadcaster != nil {
		errs = b.nexusBroadcaster.BroadcastEvent(ctx, sessionKey, event)
	}
	b.broadcastAppServerNotification(ctx, sessionKey, event)
	return errs
}

func (b *goalEventBroadcaster) broadcastAppServerNotification(ctx context.Context, sessionKey string, event protocol.EventMessage) {
	if b.rpcSubscribers == nil || event.Data == nil || event.Data["source"] == string(protocol.GoalUpdateSourceExternal) {
		return
	}
	goal, ok := event.Data["goal"].(protocol.Goal)
	if !ok {
		return
	}
	threadID := goal.SessionKey
	if threadID == "" {
		threadID = sessionKey
	}
	switch event.EventType {
	case protocol.EventTypeGoalCleared:
		b.rpcSubscribers.Broadcast(ctx, threadID, nil, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: goalappserver.ThreadGoalClearedNotification{
				ThreadID: threadID,
			},
		})
	case protocol.EventTypeGoalStatusChanged:
		if protocol.NormalizeGoalStatus(goal.Status) == protocol.GoalStatusComplete {
			b.rpcSubscribers.Broadcast(ctx, threadID, nil, goalappserver.AppServerJSONRPCNotification{
				Method: "thread/goal/cleared",
				Params: goalappserver.ThreadGoalClearedNotification{
					ThreadID: threadID,
				},
			})
			return
		}
		fallthrough
	case protocol.EventTypeGoalCreated,
		protocol.EventTypeGoalUpdated,
		protocol.EventTypeGoalProgress,
		protocol.EventTypeGoalContinuation:
		b.rpcSubscribers.Broadcast(ctx, threadID, nil, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/updated",
			Params: goalappserver.ThreadGoalUpdatedNotification{
				ThreadID: threadID,
				TurnID:   goalEventTurnID(event),
				Goal:     goalappserver.ThreadGoalFromGoal(goal),
			},
		})
	}
}

func goalEventTurnID(event protocol.EventMessage) *string {
	if event.Data == nil {
		return nil
	}
	value, _ := event.Data["round_id"].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
