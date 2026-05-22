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

type appServerGoalRPCRegistry struct {
	mu       sync.RWMutex
	threads  map[string]map[string]rawJSONSender
	senderTo map[string]map[string]struct{}
}

func newAppServerGoalRPCRegistry() *appServerGoalRPCRegistry {
	return &appServerGoalRPCRegistry{
		threads:  make(map[string]map[string]rawJSONSender),
		senderTo: make(map[string]map[string]struct{}),
	}
}

func (r *appServerGoalRPCRegistry) Register(threadID string, sender rawJSONSender) {
	threadID = strings.TrimSpace(threadID)
	if r == nil || threadID == "" || sender == nil {
		return
	}
	senderKey := sender.Key()
	if senderKey == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	senders := r.threads[threadID]
	if senders == nil {
		senders = make(map[string]rawJSONSender)
		r.threads[threadID] = senders
	}
	senders[senderKey] = sender

	threads := r.senderTo[senderKey]
	if threads == nil {
		threads = make(map[string]struct{})
		r.senderTo[senderKey] = threads
	}
	threads[threadID] = struct{}{}
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
	for threadID := range r.senderTo[senderKey] {
		delete(r.threads[threadID], senderKey)
		if len(r.threads[threadID]) == 0 {
			delete(r.threads, threadID)
		}
	}
	delete(r.senderTo, senderKey)
}

func (r *appServerGoalRPCRegistry) Broadcast(ctx context.Context, threadID string, current rawJSONSender, payload any) {
	senders := r.senders(threadID, current)
	for _, sender := range senders {
		_ = sender.SendJSON(ctx, payload)
	}
}

func (r *appServerGoalRPCRegistry) senders(threadID string, current rawJSONSender) []rawJSONSender {
	threadID = strings.TrimSpace(threadID)
	seen := make(map[string]struct{})
	result := make([]rawJSONSender, 0, 4)
	if current != nil && current.Key() != "" {
		seen[current.Key()] = struct{}{}
		result = append(result, current)
	}
	if r == nil || threadID == "" {
		return result
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, sender := range r.threads[threadID] {
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
