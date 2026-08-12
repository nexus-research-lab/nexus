// INPUT: authenticated owner identity, structured thread ID and live app-server WebSocket sender.
// OUTPUT: owner-and-thread scoped Goal notification subscriptions with sender lifecycle cleanup.
// POS: app-server Goal RPC subscription registry; authorization must succeed before registration.
package websocket

import (
	"context"
	"strings"
	"sync"
)

type rawJSONSender interface {
	Key() string
	SendJSON(context.Context, any) error
}

type appServerGoalRPCScope struct {
	ownerUserID string
	threadID    string
}

type appServerGoalRPCRegistry struct {
	mu       sync.RWMutex
	threads  map[appServerGoalRPCScope]map[string]rawJSONSender
	senderTo map[string]map[appServerGoalRPCScope]struct{}
}

func newAppServerGoalRPCRegistry() *appServerGoalRPCRegistry {
	return &appServerGoalRPCRegistry{
		threads:  make(map[appServerGoalRPCScope]map[string]rawJSONSender),
		senderTo: make(map[string]map[appServerGoalRPCScope]struct{}),
	}
}

func (r *appServerGoalRPCRegistry) Register(ownerUserID, threadID string, sender rawJSONSender) {
	scope := normalizeAppServerGoalRPCScope(ownerUserID, threadID)
	if r == nil || scope.ownerUserID == "" || scope.threadID == "" || sender == nil {
		return
	}
	senderKey := sender.Key()
	if senderKey == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	senders := r.threads[scope]
	if senders == nil {
		senders = make(map[string]rawJSONSender)
		r.threads[scope] = senders
	}
	senders[senderKey] = sender

	threads := r.senderTo[senderKey]
	if threads == nil {
		threads = make(map[appServerGoalRPCScope]struct{})
		r.senderTo[senderKey] = threads
	}
	threads[scope] = struct{}{}
}

func (r *appServerGoalRPCRegistry) UnregisterSender(sender rawJSONSender) {
	if r == nil || sender == nil {
		return
	}
	senderKey := sender.Key()
	if senderKey == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for scope := range r.senderTo[senderKey] {
		delete(r.threads[scope], senderKey)
		if len(r.threads[scope]) == 0 {
			delete(r.threads, scope)
		}
	}
	delete(r.senderTo, senderKey)
}

func (r *appServerGoalRPCRegistry) Broadcast(
	ctx context.Context,
	ownerUserID string,
	threadID string,
	current rawJSONSender,
	payload any,
) {
	senders := r.senders(ownerUserID, threadID, current)
	for _, sender := range senders {
		_ = sender.SendJSON(ctx, payload)
	}
}

func (r *appServerGoalRPCRegistry) senders(
	ownerUserID string,
	threadID string,
	current rawJSONSender,
) []rawJSONSender {
	scope := normalizeAppServerGoalRPCScope(ownerUserID, threadID)
	seen := make(map[string]struct{})
	result := make([]rawJSONSender, 0, 4)
	if current != nil && current.Key() != "" {
		seen[current.Key()] = struct{}{}
		result = append(result, current)
	}
	if r == nil || scope.ownerUserID == "" || scope.threadID == "" {
		return result
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, sender := range r.threads[scope] {
		if sender == nil {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, sender)
	}
	return result
}

func normalizeAppServerGoalRPCScope(ownerUserID, threadID string) appServerGoalRPCScope {
	return appServerGoalRPCScope{
		ownerUserID: strings.TrimSpace(ownerUserID),
		threadID:    strings.TrimSpace(threadID),
	}
}
