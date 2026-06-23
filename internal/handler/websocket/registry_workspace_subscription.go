package websocket

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

// RuntimeSnapshot 描述某个 agent 当前的运行态快照。
type RuntimeSnapshot struct {
	AgentID          string `json:"agent_id"`
	RunningTaskCount int    `json:"running_task_count"`
	Status           string `json:"status"`
}

type workspaceEventSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type runtimeSnapshotProvider func(string) RuntimeSnapshot

type workspaceSenderSubscription struct {
	refCount          int
	token             string
	watchFileRefCount int
}

type workspaceSubscriptionRegistry struct {
	mu              sync.Mutex
	workspace       *workspacesvc.Service
	runtimeProvider runtimeSnapshotProvider
	senderTokens    map[string]map[string]workspaceSenderSubscription
	agentSenders    map[string]map[string]workspaceEventSender
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
		senderTokens:    make(map[string]map[string]workspaceSenderSubscription),
		agentSenders:    make(map[string]map[string]workspaceEventSender),
		lastSnapshots:   make(map[string]RuntimeSnapshot),
	}
}

func (r *workspaceSubscriptionRegistry) Subscribe(ctx context.Context, sender workspaceEventSender, agentID string, watchFiles bool) error {
	if r == nil || sender == nil || sender.IsClosed() {
		return nil
	}

	r.mu.Lock()
	if r.senderTokens[sender.Key()] != nil {
		if subscription, exists := r.senderTokens[sender.Key()][agentID]; exists {
			subscription.refCount++
			needsLiveUpgrade := watchFiles && subscription.token == "" && r.workspace != nil
			if watchFiles {
				subscription.watchFileRefCount++
			}
			r.senderTokens[sender.Key()][agentID] = subscription
			r.mu.Unlock()
			if needsLiveUpgrade {
				token, err := r.subscribeWorkspaceLive(ctx, sender, agentID)
				if err != nil {
					r.Unsubscribe(sender, agentID, watchFiles)
					return err
				}
				r.attachLiveToken(sender.Key(), agentID, token)
			}
			r.sendRuntimeSnapshot(sender, agentID)
			return nil
		}
	}
	r.mu.Unlock()

	token := ""
	if watchFiles && r.workspace != nil {
		liveToken, err := r.subscribeWorkspaceLive(ctx, sender, agentID)
		if err != nil {
			return err
		}
		token = liveToken
	}

	r.mu.Lock()
	if sender.IsClosed() {
		r.mu.Unlock()
		if token != "" && r.workspace != nil {
			r.workspace.UnsubscribeLive(token)
		}
		return nil
	}
	if r.senderTokens[sender.Key()] == nil {
		r.senderTokens[sender.Key()] = make(map[string]workspaceSenderSubscription)
	}
	r.senderTokens[sender.Key()][agentID] = workspaceSenderSubscription{
		refCount:          1,
		token:             token,
		watchFileRefCount: boolToInt(watchFiles),
	}
	if r.agentSenders[agentID] == nil {
		r.agentSenders[agentID] = make(map[string]workspaceEventSender)
	}
	r.agentSenders[agentID][sender.Key()] = sender
	r.ensurePollerLocked()
	r.mu.Unlock()

	r.sendRuntimeSnapshot(sender, agentID)
	return nil
}

func (r *workspaceSubscriptionRegistry) subscribeWorkspaceLive(ctx context.Context, sender workspaceEventSender, agentID string) (string, error) {
	if r.workspace == nil {
		return "", nil
	}
	return r.workspace.SubscribeLive(ctx, agentID, func(event workspacesvc.LiveEvent) {
		_ = sender.SendEvent(context.Background(), workspaceEventMessage(event))
	})
}

func (r *workspaceSubscriptionRegistry) attachLiveToken(senderKey string, agentID string, token string) {
	if token == "" || r.workspace == nil {
		return
	}

	shouldRelease := false
	r.mu.Lock()
	subscription, exists := r.senderTokens[senderKey][agentID]
	if !exists || subscription.token != "" || subscription.watchFileRefCount == 0 {
		shouldRelease = true
	} else {
		subscription.token = token
		r.senderTokens[senderKey][agentID] = subscription
	}
	r.mu.Unlock()

	if shouldRelease {
		r.workspace.UnsubscribeLive(token)
	}
}

func (r *workspaceSubscriptionRegistry) Unsubscribe(sender workspaceEventSender, agentID string, watchFiles bool) {
	if r == nil || sender == nil {
		return
	}
	r.unsubscribe(sender.Key(), agentID, watchFiles)
}

func (r *workspaceSubscriptionRegistry) UnregisterSender(sender workspaceEventSender) {
	if r == nil || sender == nil {
		return
	}
	r.mu.Lock()
	agentTokens := r.senderTokens[sender.Key()]
	agentIDs := slices.Sorted(maps.Keys(agentTokens))
	r.mu.Unlock()

	for _, agentID := range agentIDs {
		r.remove(sender.Key(), agentID)
	}
}

func (r *workspaceSubscriptionRegistry) unsubscribe(senderKey string, agentID string, watchFiles bool) {
	r.mu.Lock()
	agentTokens := r.senderTokens[senderKey]
	if agentTokens == nil {
		r.mu.Unlock()
		return
	}
	subscription := agentTokens[agentID]
	tokenToRelease := ""
	if watchFiles && subscription.watchFileRefCount > 0 {
		subscription.watchFileRefCount--
		if subscription.watchFileRefCount == 0 && subscription.refCount > 1 {
			tokenToRelease = subscription.token
			subscription.token = ""
		}
	}
	if subscription.refCount > 1 {
		subscription.refCount--
		agentTokens[agentID] = subscription
		r.mu.Unlock()
		if tokenToRelease != "" && r.workspace != nil {
			r.workspace.UnsubscribeLive(tokenToRelease)
		}
		return
	}
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

	if tokenToRelease != "" && tokenToRelease != subscription.token && r.workspace != nil {
		r.workspace.UnsubscribeLive(tokenToRelease)
	}
	if subscription.token != "" && r.workspace != nil {
		r.workspace.UnsubscribeLive(subscription.token)
	}
}

func (r *workspaceSubscriptionRegistry) remove(senderKey string, agentID string) {
	r.mu.Lock()
	agentTokens := r.senderTokens[senderKey]
	if agentTokens == nil {
		r.mu.Unlock()
		return
	}
	subscription, exists := agentTokens[agentID]
	if !exists {
		r.mu.Unlock()
		return
	}
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

	if subscription.token != "" && r.workspace != nil {
		r.workspace.UnsubscribeLive(subscription.token)
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
