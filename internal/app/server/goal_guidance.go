// INPUT: Goal steering 的 session scope 与可选 caller Agent。
// OUTPUT: DM 单 session 或 Room 每个活跃 slot 的 runtime guidance。
// POS: Goal service 到 DM/Room runtime guidance 的装配适配器。
package server

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type goalGuidanceDispatcher struct {
	runtime *runtimectx.Manager
	room    *roomsvc.RealtimeService
}

func (d goalGuidanceDispatcher) QueueGuidanceInput(ctx context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	return d.runtime.QueueGuidanceInput(ctx, sessionKey, roundID, content)
}

func (d goalGuidanceDispatcher) QueueContextualGuidanceInput(ctx context.Context, sessionKey string, roundID string, contextName string, content string, objectiveRevision int64) ([]string, error) {
	if protocol.IsRoomSharedSessionKey(sessionKey) && d.room != nil {
		return d.room.QueueRoomContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content, "", objectiveRevision)
	}
	return d.runtime.QueueContextualGuidanceInputOnConsumed(ctx, sessionKey, roundID, contextName, content, func() {
		d.runtime.AdoptGoalObjectiveRevision(sessionKey, objectiveRevision)
	})
}

func (d goalGuidanceDispatcher) QueueRoomContextualGuidanceInput(ctx context.Context, sessionKey string, roundID string, contextName string, content string, excludedAgentID string, objectiveRevision int64) ([]string, error) {
	if d.room != nil {
		return d.room.QueueRoomContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content, excludedAgentID, objectiveRevision)
	}
	return d.runtime.QueueContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content)
}
