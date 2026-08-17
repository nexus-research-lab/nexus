package websocket

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *workspaceSubscriptionRegistry) sendRuntimeSnapshot(sender workspaceEventSender, agentID string) {
	if r == nil || r.runtimeProvider == nil || sender == nil || sender.IsClosed() {
		return
	}
	_ = sender.SendEvent(context.Background(), runtimeSnapshotEvent(r.runtimeProvider(agentID)))
}

func runtimeSnapshotEvent(snapshot RuntimeSnapshot) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeAgentRuntimeEvent, map[string]any{
		"agent_id":           snapshot.AgentID,
		"running_task_count": snapshot.RunningTaskCount,
		"status":             snapshot.Status,
	})
	event.AgentID = snapshot.AgentID
	return event
}
