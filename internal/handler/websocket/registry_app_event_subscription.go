package websocket

import (
	"context"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type appEventSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type appEventSubscription struct {
	refCount int
	sender   appEventSender
}

// appEventSubscriptionRegistry 负责全局应用事件订阅，例如目录和定时任务变更。
type appEventSubscriptionRegistry struct {
	mu          sync.Mutex
	subscribers map[string]appEventSubscription
}

func newAppEventSubscriptionRegistry() *appEventSubscriptionRegistry {
	return &appEventSubscriptionRegistry{
		subscribers: make(map[string]appEventSubscription),
	}
}

func (r *appEventSubscriptionRegistry) Subscribe(sender appEventSender) {
	if r == nil || sender == nil || sender.IsClosed() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	subscription := r.subscribers[sender.Key()]
	subscription.refCount++
	subscription.sender = sender
	r.subscribers[sender.Key()] = subscription
}

func (r *appEventSubscriptionRegistry) Unsubscribe(sender appEventSender) {
	if r == nil || sender == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	subscription, exists := r.subscribers[sender.Key()]
	if !exists {
		return
	}
	if subscription.refCount > 1 {
		subscription.refCount--
		r.subscribers[sender.Key()] = subscription
		return
	}
	delete(r.subscribers, sender.Key())
}

func (r *appEventSubscriptionRegistry) UnregisterSender(sender appEventSender) {
	if r == nil || sender == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subscribers, sender.Key())
}

func (r *appEventSubscriptionRegistry) Broadcast(ctx context.Context, event protocol.EventMessage) []error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	senders := make([]appEventSender, 0, len(r.subscribers))
	for _, subscription := range r.subscribers {
		if subscription.sender != nil && !subscription.sender.IsClosed() {
			senders = append(senders, subscription.sender)
		}
	}
	r.mu.Unlock()

	errs := make([]error, 0)
	for _, sender := range senders {
		if err := sender.SendEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
