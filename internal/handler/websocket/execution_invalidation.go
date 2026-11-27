// INPUT: orchestration 的 owner/session-scoped 失效事实与已认证 bind_session sender。
// OUTPUT: 仅向同 owner + session 连接投递 execution_invalidated 事件；同一 scope 的
// 连续失效按窗口合并为“首个立即 + 窗口末尾至多一个”。
// POS: ExecutionInvalidationSink 的 WebSocket adapter；不拥有或修改 WorkGraph 状态。
package websocket

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionInvalidationSender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

// executionInvalidationWindow 限制每个 owner/session 每秒至多一次 trailing 刷新。
// 一个运行中的 WorkGraph 每个工具事件都会失效，逐条转发会让每个打开的页面按
// 工具频率重读整图；事件只是“请重读”的信号，合并后不丢失最终状态。
const executionInvalidationWindow = time.Second

type executionInvalidationRegistry struct {
	mu           sync.Mutex
	byScope      map[string]map[string]executionInvalidationSender
	senderScopes map[string]map[string]struct{}
	window       time.Duration
	// throttles 只记录窗口内仍活跃的 scope；空窗口结束即删除。
	throttles map[string]*executionInvalidationThrottle
}

type executionInvalidationThrottle struct {
	pending *orchestrationsvc.ExecutionInvalidation
}

func newExecutionInvalidationRegistry() *executionInvalidationRegistry {
	return &executionInvalidationRegistry{
		byScope:      make(map[string]map[string]executionInvalidationSender),
		senderScopes: make(map[string]map[string]struct{}),
		window:       executionInvalidationWindow,
		throttles:    make(map[string]*executionInvalidationThrottle),
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

// Invalidate 合并同一 scope 的失效：窗口空闲时立即投递，窗口内只保留最新一条，
// 窗口结束时补发。trailing 投递脱离触发它的请求 ctx，避免请求结束吞掉最后状态。
func (r *executionInvalidationRegistry) Invalidate(
	ctx context.Context,
	invalidation orchestrationsvc.ExecutionInvalidation,
) {
	if r == nil {
		return
	}
	scopeKey := executionInvalidationScopeKey(invalidation.OwnerUserID, invalidation.SessionKey)
	if scopeKey == "" {
		return
	}
	r.mu.Lock()
	if throttle := r.throttles[scopeKey]; throttle != nil {
		throttle.pending = &invalidation
		r.mu.Unlock()
		return
	}
	r.throttles[scopeKey] = &executionInvalidationThrottle{}
	r.mu.Unlock()
	time.AfterFunc(r.window, func() { r.flush(scopeKey) })
	r.broadcastInvalidation(ctx, invalidation)
}

func (r *executionInvalidationRegistry) flush(scopeKey string) {
	r.mu.Lock()
	throttle := r.throttles[scopeKey]
	if throttle == nil || throttle.pending == nil {
		delete(r.throttles, scopeKey)
		r.mu.Unlock()
		return
	}
	invalidation := *throttle.pending
	throttle.pending = nil
	r.mu.Unlock()
	// 补发后再开一个窗口：持续写入时保持每窗口一次，停止后下一窗口自然清理。
	time.AfterFunc(r.window, func() { r.flush(scopeKey) })
	r.broadcastInvalidation(context.Background(), invalidation)
}

func (r *executionInvalidationRegistry) broadcastInvalidation(
	ctx context.Context,
	invalidation orchestrationsvc.ExecutionInvalidation,
) {
	event := protocol.NewExecutionInvalidatedEvent(
		invalidation.SessionKey,
		protocol.ExecutionInvalidationData{
			ExecutionID: invalidation.ExecutionID,
			Version:     invalidation.Version,
		},
	)
	r.Broadcast(ctx, invalidation.OwnerUserID, invalidation.SessionKey, event)
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
	h.executionInvalidations.Invalidate(ctx, invalidation)
}

// sendExecutionInvalidationFence makes bind/rebind a read-after-connect fence.
// It bypasses the scope throttle: only this sender needs it and it must not wait.
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
