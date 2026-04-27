package websocket

import (
	"context"
	"sync"
	"time"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

// RuntimeSnapshot 描述某个 agent 当前的运行态快照。
type RuntimeSnapshot struct {
	AgentID          string `json:"agent_id"`
	RunningTaskCount int    `json:"running_task_count"`
	Status           string `json:"status"`
}

type runtimeSnapshotProvider func(string) RuntimeSnapshot

type workspaceSubscriptionRegistry struct {
	mu              sync.Mutex
	workspace       *workspacesvc.Service
	runtimeProvider runtimeSnapshotProvider
	senderTokens    map[string]map[string]string
	agentSenders    map[string]map[string]*gatewayshared.WebSocketSender
	lastSnapshots   map[string]RuntimeSnapshot
	pollerCancel    context.CancelFunc
}

func newWorkspaceSubscriptionRegistry(
	workspaceService *workspacesvc.Service,
	runtimeProvider runtimeSnapshotProvider,
) *workspaceSubscriptionRegistry {
	return &workspaceSubscriptionRegistry{
		workspace:       workspaceService,
		runtimeProvider: runtimeProvider,
		senderTokens:    make(map[string]map[string]string),
		agentSenders:    make(map[string]map[string]*gatewayshared.WebSocketSender),
		lastSnapshots:   make(map[string]RuntimeSnapshot),
	}
}

func (r *workspaceSubscriptionRegistry) Subscribe(ctx context.Context, sender *gatewayshared.WebSocketSender, agentID string) error {
	if r == nil || r.workspace == nil || sender == nil || sender.IsClosed() {
		return nil
	}

	r.mu.Lock()
	if r.senderTokens[sender.Key()] != nil && r.senderTokens[sender.Key()][agentID] != "" {
		r.mu.Unlock()
		r.sendRuntimeSnapshot(sender, agentID)
		return nil
	}
	r.mu.Unlock()

	token, err := r.workspace.SubscribeLive(ctx, agentID, func(event workspacesvc.LiveEvent) {
		_ = sender.SendEvent(context.Background(), workspaceEventMessage(event))
	})
	if err != nil {
		return err
	}

	r.mu.Lock()
	if sender.IsClosed() {
		r.mu.Unlock()
		r.workspace.UnsubscribeLive(token)
		return nil
	}
	if r.senderTokens[sender.Key()] == nil {
		r.senderTokens[sender.Key()] = make(map[string]string)
	}
	r.senderTokens[sender.Key()][agentID] = token
	if r.agentSenders[agentID] == nil {
		r.agentSenders[agentID] = make(map[string]*gatewayshared.WebSocketSender)
	}
	r.agentSenders[agentID][sender.Key()] = sender
	r.ensurePollerLocked()
	r.mu.Unlock()

	r.sendRuntimeSnapshot(sender, agentID)
	return nil
}

func (r *workspaceSubscriptionRegistry) Unsubscribe(sender *gatewayshared.WebSocketSender, agentID string) {
	if r == nil || sender == nil {
		return
	}
	r.unsubscribe(sender.Key(), agentID)
}

func (r *workspaceSubscriptionRegistry) UnregisterSender(sender *gatewayshared.WebSocketSender) {
	if r == nil || sender == nil {
		return
	}
	r.mu.Lock()
	agentTokens := r.senderTokens[sender.Key()]
	agentIDs := make([]string, 0, len(agentTokens))
	for agentID := range agentTokens {
		agentIDs = append(agentIDs, agentID)
	}
	r.mu.Unlock()

	for _, agentID := range agentIDs {
		r.unsubscribe(sender.Key(), agentID)
	}
}

func (r *workspaceSubscriptionRegistry) unsubscribe(senderKey string, agentID string) {
	r.mu.Lock()
	agentTokens := r.senderTokens[senderKey]
	if agentTokens == nil {
		r.mu.Unlock()
		return
	}
	token := agentTokens[agentID]
	delete(agentTokens, agentID)
	if len(agentTokens) == 0 {
		delete(r.senderTokens, senderKey)
	}
	if senders := r.agentSenders[agentID]; senders != nil {
		delete(senders, senderKey)
		if len(senders) == 0 {
			delete(r.agentSenders, agentID)
			delete(r.lastSnapshots, agentID)
		}
	}
	if len(r.agentSenders) == 0 && r.pollerCancel != nil {
		r.pollerCancel()
		r.pollerCancel = nil
	}
	r.mu.Unlock()

	if token != "" {
		r.workspace.UnsubscribeLive(token)
	}
}

func (r *workspaceSubscriptionRegistry) ensurePollerLocked() {
	if r.pollerCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.pollerCancel = cancel
	go r.runPoller(ctx)
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

	type broadcast struct {
		senders []*gatewayshared.WebSocketSender
		event   protocol.EventMessage
	}

	pending := make([]broadcast, 0)
	r.mu.Lock()
	for agentID, senders := range r.agentSenders {
		snapshot := r.runtimeProvider(agentID)
		previous, exists := r.lastSnapshots[agentID]
		if exists && previous == snapshot {
			continue
		}
		r.lastSnapshots[agentID] = snapshot
		targets := make([]*gatewayshared.WebSocketSender, 0, len(senders))
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
	}
	r.mu.Unlock()

	for _, item := range pending {
		for _, sender := range item.senders {
			_ = sender.SendEvent(context.Background(), item.event)
		}
	}
}

func (r *workspaceSubscriptionRegistry) sendRuntimeSnapshot(sender *gatewayshared.WebSocketSender, agentID string) {
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

func workspaceEventMessage(event workspacesvc.LiveEvent) protocol.EventMessage {
	data := map[string]any{
		"type":      event.Type,
		"agent_id":  event.AgentID,
		"path":      event.Path,
		"version":   event.Version,
		"source":    event.Source,
		"timestamp": event.Timestamp,
	}
	if event.SessionKey != nil {
		data["session_key"] = *event.SessionKey
	}
	if event.ToolUseID != nil {
		data["tool_use_id"] = *event.ToolUseID
	}
	if event.ContentSnapshot != nil {
		data["content_snapshot"] = *event.ContentSnapshot
	}
	if event.AppendedText != nil {
		data["appended_text"] = *event.AppendedText
	}
	if event.DiffStats != nil {
		data["diff_stats"] = map[string]any{
			"additions":     event.DiffStats.Additions,
			"deletions":     event.DiffStats.Deletions,
			"changed_lines": event.DiffStats.ChangedLines,
		}
	}

	message := protocol.NewEvent(protocol.EventTypeWorkspaceEvent, data)
	message.AgentID = event.AgentID
	if event.SessionKey != nil {
		message.SessionKey = *event.SessionKey
	}
	return message
}
