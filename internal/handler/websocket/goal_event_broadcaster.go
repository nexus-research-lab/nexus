// INPUT: Goal 领域事件、owner-scoped session/thread identity 与 Nexus/app-server 订阅表。
// OUTPUT: Nexus 事件广播及 owner 隔离的 thread/goal updated/cleared JSON-RPC 通知。
// POS: Goal durable event 到 WebSocket 两种协议面的唯一通知投影；不向 ownerless 或跨 owner 订阅者广播。
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
	ownerUserID := protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataOwnerUserID)
	if ownerUserID == "" {
		return
	}
	if protocol.NormalizeGoalStatus(goal.Status) == protocol.GoalStatusComplete {
		// 完成后的 usage 结算仍写旧 Goal，但 app-server 通知只有 threadId，
		// 无法区分 Goal generation。若此时已有新 Goal，重发 cleared 会误清新 Goal。
		if event.EventType == protocol.EventTypeGoalStatusChanged ||
			event.EventType == protocol.EventTypeGoalCleared {
			b.rpcSubscribers.Broadcast(ctx, ownerUserID, threadID, nil, goalappserver.AppServerJSONRPCNotification{
				Method: "thread/goal/cleared",
				Params: goalappserver.ThreadGoalClearedNotification{
					ThreadID: threadID,
				},
			})
		}
		return
	}
	switch event.EventType {
	case protocol.EventTypeGoalCleared:
		b.rpcSubscribers.Broadcast(ctx, ownerUserID, threadID, nil, goalappserver.AppServerJSONRPCNotification{
			Method: "thread/goal/cleared",
			Params: goalappserver.ThreadGoalClearedNotification{
				ThreadID: threadID,
			},
		})
	case protocol.EventTypeGoalStatusChanged:
		fallthrough
	case protocol.EventTypeGoalCreated,
		protocol.EventTypeGoalUpdated,
		protocol.EventTypeGoalProgress,
		protocol.EventTypeGoalContinuation:
		b.rpcSubscribers.Broadcast(ctx, ownerUserID, threadID, nil, goalappserver.AppServerJSONRPCNotification{
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
