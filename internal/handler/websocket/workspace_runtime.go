package websocket

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeBroadcast struct {
	agentID string
	senders []workspaceEventSender
	event   protocol.EventMessage
	flush   bool
}

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

	pending := r.collectRuntimeBroadcasts(r.subscribedRuntimeAgents())
	r.flushIdleRuntimeAgents(pending)
	sendRuntimeBroadcasts(pending)
}

func (r *workspaceSubscriptionRegistry) subscribedRuntimeAgents() map[string][]workspaceEventSender {
	r.mu.Lock()
	defer r.mu.Unlock()

	agentSenders := make(map[string][]workspaceEventSender, len(r.agentSenders))
	for agentID, senders := range r.agentSenders {
		agentSenders[agentID] = slices.Collect(maps.Values(senders))
	}
	return agentSenders
}

func (r *workspaceSubscriptionRegistry) collectRuntimeBroadcasts(
	agentSenders map[string][]workspaceEventSender,
) []runtimeBroadcast {
	pending := make([]runtimeBroadcast, 0, len(agentSenders))
	for agentID, senders := range agentSenders {
		snapshot := r.runtimeProvider(agentID)
		if !r.recordRuntimeSnapshot(agentID, snapshot) {
			continue
		}
		targets := openWorkspaceSenders(senders)
		if len(targets) == 0 {
			continue
		}
		pending = append(pending, runtimeBroadcast{
			agentID: agentID,
			senders: targets,
			event:   runtimeSnapshotEvent(snapshot),
			flush:   snapshot.RunningTaskCount == 0 && snapshot.Status != "running",
		})
	}
	return pending
}

func (r *workspaceSubscriptionRegistry) recordRuntimeSnapshot(agentID string, snapshot RuntimeSnapshot) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.agentSenders[agentID]) == 0 {
		return false
	}
	previous, exists := r.lastSnapshots[agentID]
	if exists && previous == snapshot {
		return false
	}
	r.lastSnapshots[agentID] = snapshot
	return true
}

func openWorkspaceSenders(senders []workspaceEventSender) []workspaceEventSender {
	targets := make([]workspaceEventSender, 0, len(senders))
	for _, sender := range senders {
		if sender != nil && !sender.IsClosed() {
			targets = append(targets, sender)
		}
	}
	return targets
}

func (r *workspaceSubscriptionRegistry) flushIdleRuntimeAgents(pending []runtimeBroadcast) {
	if r.workspace == nil {
		return
	}
	for _, item := range pending {
		if item.flush {
			r.workspace.FlushLiveWrites(item.agentID)
		}
	}
}

func sendRuntimeBroadcasts(pending []runtimeBroadcast) {
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
