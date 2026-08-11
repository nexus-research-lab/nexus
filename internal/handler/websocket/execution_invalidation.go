// INPUT: orchestration 的 owner/session-scoped 失效事实与已认证 bind_session sender。
// OUTPUT: 仅向同 owner + session 连接投递 execution_invalidated 事件。
// POS: ExecutionInvalidationSink 的 WebSocket adapter；不拥有或修改 WorkGraph 状态。
package websocket

import (
	"context"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionInvalidationSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type executionInvalidationRegistry struct {
	mu           sync.Mutex
	byScope      map[string]map[string]executionInvalidationSender
	senderScopes map[string]map[string]struct{}
}

func newExecutionInvalidationRegistry() *executionInvalidationRegistry {
	return &executionInvalidationRegistry{
		byScope:      make(map[string]map[string]executionInvalidationSender),
		senderScopes: make(map[string]map[string]struct{}),
	}
}

func executionInvalidationScopeKey(ownerUserID string, sessionKey string) string {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return ""
	}
	return ownerUserID + "\x00" + sessionKey
}

func (r *executionInvalidationRegistry) Bind(
	ownerUserID string,
	sessionKey string,
	sender executionInvalidationSender,
) {
	if r == nil || sender == nil || sender.IsClosed() {
		return
	}
	scopeKey := executionInvalidationScopeKey(ownerUserID, sessionKey)
	if scopeKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byScope[scopeKey] == nil {
		r.byScope[scopeKey] = make(map[string]executionInvalidationSender)
	}
	r.byScope[scopeKey][sender.Key()] = sender
	if r.senderScopes[sender.Key()] == nil {
		r.senderScopes[sender.Key()] = make(map[string]struct{})
	}
	r.senderScopes[sender.Key()][scopeKey] = struct{}{}
}

func (r *executionInvalidationRegistry) Unbind(
	ownerUserID string,
	sessionKey string,
	sender executionInvalidationSender,
) {
	if r == nil || sender == nil {
		return
	}
	scopeKey := executionInvalidationScopeKey(ownerUserID, sessionKey)
	if scopeKey == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeLocked(scopeKey, sender.Key())
}

func (r *executionInvalidationRegistry) UnregisterSender(
	sender executionInvalidationSender,
) {
	if r == nil || sender == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	senderKey := sender.Key()
	for scopeKey := range r.senderScopes[senderKey] {
		delete(r.byScope[scopeKey], senderKey)
		if len(r.byScope[scopeKey]) == 0 {
			delete(r.byScope, scopeKey)
		}
	}
	delete(r.senderScopes, senderKey)
}

func (r *executionInvalidationRegistry) Broadcast(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	event protocol.EventMessage,
) []error {
	if r == nil {
		return nil
	}
	scopeKey := executionInvalidationScopeKey(ownerUserID, sessionKey)
	if scopeKey == "" {
		return nil
	}
	r.mu.Lock()
	senders := make([]executionInvalidationSender, 0, len(r.byScope[scopeKey]))
	for senderKey, sender := range r.byScope[scopeKey] {
		if sender == nil || sender.IsClosed() {
			r.removeLocked(scopeKey, senderKey)
			continue
		}
		senders = append(senders, sender)
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

func (r *executionInvalidationRegistry) removeLocked(scopeKey string, senderKey string) {
	delete(r.byScope[scopeKey], senderKey)
	if len(r.byScope[scopeKey]) == 0 {
		delete(r.byScope, scopeKey)
	}
	delete(r.senderScopes[senderKey], scopeKey)
	if len(r.senderScopes[senderKey]) == 0 {
		delete(r.senderScopes, senderKey)
	}
}

// InvalidateExecution 实现 orchestration 的窄失效事件 port。
func (h *Handler) InvalidateExecution(
	ctx context.Context,
	invalidation orchestrationsvc.ExecutionInvalidation,
) {
	if h == nil || h.executionInvalidations == nil {
		return
	}
	event := protocol.NewExecutionInvalidatedEvent(
		invalidation.SessionKey,
		protocol.ExecutionInvalidationData{
			ExecutionID: invalidation.ExecutionID,
			Version:     invalidation.Version,
		},
	)
	h.executionInvalidations.Broadcast(
		ctx,
		invalidation.OwnerUserID,
		invalidation.SessionKey,
		event,
	)
}

// sendExecutionInvalidationFence makes bind/rebind a read-after-connect fence.
// Empty Execution identity is intentional: the client must read the current
// owner/session projection, including the authoritative "no graph" result.
func sendExecutionInvalidationFence(
	ctx context.Context,
	sender sessionEventSender,
	sessionKey string,
) error {
	if sender == nil || strings.TrimSpace(sessionKey) == "" {
		return nil
	}
	return sender.SendEvent(
		ctx,
		protocol.NewExecutionInvalidatedEvent(
			sessionKey,
			protocol.ExecutionInvalidationData{},
		),
	)
}
