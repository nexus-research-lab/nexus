package websocket

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *workspaceSubscriptionRegistry) runPoller(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.broadcastRuntimeChanges()
		}
	}
}

func (r *workspaceSubscriptionRegistry) broadcastRuntimeChanges() {
	if r == nil || r.runtimeProvider == nil {
		return
	}

	type broadcast struct {
		senders []workspaceEventSender
		event   protocol.EventMessage
	}

	pending := make([]broadcast, 0)
	flushAgentIDs := make([]string, 0)
	r.mu.Lock()
	for agentID, senders := range r.agentSenders {
		snapshot := r.runtimeProvider(agentID)
		previous, exists := r.lastSnapshots[agentID]
		if exists && previous == snapshot {
			continue
		}
		r.lastSnapshots[agentID] = snapshot
		targets := make([]workspaceEventSender, 0, len(senders))
		for _, sender := range senders {
			if sender != nil && !sender.IsClosed() {
				targets = append(targets, sender)
			}
		}
		if len(targets) == 0 {
			continue
		}
		pending = append(pending, broadcast{
			senders: targets,
			event:   runtimeSnapshotEvent(snapshot),
		})
		if snapshot.RunningTaskCount == 0 && snapshot.Status != "running" {
			flushAgentIDs = append(flushAgentIDs, agentID)
		}
	}
	r.mu.Unlock()

	if r.workspace != nil {
		for _, agentID := range flushAgentIDs {
			r.workspace.FlushLiveWrites(agentID)
		}
	}

	for _, item := range pending {
		for _, sender := range item.senders {
			_ = sender.SendEvent(context.Background(), item.event)
		}
	}
}

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
