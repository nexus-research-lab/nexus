package permission

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// Sender 抽象出 WebSocket 级别的事件发送能力。
type Sender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

type senderBinding struct {
	Sender Sender
}

// Context 保存 session 绑定与权限请求广播逻辑。
type Context struct {
	mu              sync.RWMutex
	sessionBindings map[string]map[string]senderBinding
	senderSessions  map[string]map[string]struct{}
	sessionRoutes   map[string]RouteContext
	pendingRequests map[string]*PendingRequest
	requestTimeout  time.Duration
}

// NewContext 创建权限运行时上下文。
func NewContext() *Context {
	return &Context{
		sessionBindings: make(map[string]map[string]senderBinding),
		senderSessions:  make(map[string]map[string]struct{}),
		sessionRoutes:   make(map[string]RouteContext),
		pendingRequests: make(map[string]*PendingRequest),
		requestTimeout:  time.Minute,
	}
}

// BindSession 绑定 sender 到 session。
func (c *Context) BindSession(sessionKey string, sender Sender) {
	if sender == nil || sender.IsClosed() || sessionKey == "" {
		return
	}

	c.mu.Lock()
	bindings := c.sessionBindings[sessionKey]
	if bindings == nil {
		bindings = make(map[string]senderBinding)
		c.sessionBindings[sessionKey] = bindings
	}
	senderKey := sender.Key()
	bindings[senderKey] = senderBinding{
		Sender: sender,
	}

	sessions := c.senderSessions[senderKey]
	if sessions == nil {
		sessions = make(map[string]struct{})
		c.senderSessions[senderKey] = sessions
	}
	sessions[sessionKey] = struct{}{}

	c.pruneClosedBindingsLocked(sessionKey)
	c.mu.Unlock()

	go c.replayPendingRequestsToSender(sessionKey, sender)
}

// UnbindSession 解绑 sender 对指定 session 的绑定。
func (c *Context) UnbindSession(sessionKey string, sender Sender) {
	if sender == nil || sessionKey == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeBindingLocked(sessionKey, sender.Key())
	c.pruneClosedBindingsLocked(sessionKey)
}

// UnregisterSender 删除 sender 持有的全部绑定。
func (c *Context) UnregisterSender(sender Sender) []string {
	if sender == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	senderKey := sender.Key()
	sessions := c.senderSessions[senderKey]
	if len(sessions) == 0 {
		return nil
	}

	changed := make([]string, 0, len(sessions))
	for sessionKey := range sessions {
		c.removeBindingLocked(sessionKey, senderKey)
		c.pruneClosedBindingsLocked(sessionKey)
		changed = append(changed, sessionKey)
	}
	delete(c.senderSessions, senderKey)
	sort.Strings(changed)
	return changed
}

// IsBound 判断 sender 是否已绑定到 session。
func (c *Context) IsBound(sessionKey string, sender Sender) bool {
	if sender == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneClosedBindingsLocked(sessionKey)
	_, ok := c.sessionBindings[sessionKey][sender.Key()]
	return ok
}

// ResolveSessionSenders 返回当前 session 的全部绑定 sender。
func (c *Context) ResolveSessionSenders(sessionKey string) []Sender {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneClosedBindingsLocked(sessionKey)
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		return nil
	}
	result := make([]Sender, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Sender == nil || binding.Sender.IsClosed() {
			continue
		}
		result = append(result, binding.Sender)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].Key() < result[j].Key()
	})
	return result
}

// BroadcastSessionStatus 向当前 session 的全部绑定连接广播 session_status。
func (c *Context) BroadcastSessionStatus(ctx context.Context, sessionKey string, runningRoundIDs []string) []error {
	senders := c.ResolveSessionSenders(sessionKey)
	if len(senders) == 0 {
		return nil
	}
	event := protocol.NewEvent(protocol.EventTypeSessionStatus, map[string]any{
		"is_generating":     len(runningRoundIDs) > 0,
		"running_round_ids": runningRoundIDs,
	})
	event.SessionKey = sessionKey

	errs := make([]error, 0)
	for _, sender := range senders {
		if err := sender.SendEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// BroadcastEvent 向某个 session 的全部绑定连接广播通用事件。
func (c *Context) BroadcastEvent(ctx context.Context, sessionKey string, event protocol.EventMessage) []error {
	senders := c.ResolveSessionSenders(sessionKey)
	if len(senders) == 0 {
		return nil
	}
	if event.SessionKey == "" {
		event.SessionKey = sessionKey
	}
	errs := make([]error, 0)
	for _, sender := range senders {
		if err := sender.SendEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// BindSessionRoute 记录运行时 session 到前端路由 session 的映射。
func (c *Context) BindSessionRoute(sessionKey string, route RouteContext) {
	if sessionKey == "" {
		return
	}
	if route.DispatchSessionKey == "" {
		route.DispatchSessionKey = sessionKey
	}
	c.mu.Lock()
	c.sessionRoutes[sessionKey] = route
	c.mu.Unlock()
}

// UnbindSessionRoute 移除运行时 session 路由映射。
func (c *Context) UnbindSessionRoute(sessionKey string) {
	if sessionKey == "" {
		return
	}
	c.mu.Lock()
	delete(c.sessionRoutes, sessionKey)
	c.mu.Unlock()
}

// ResolveDispatchSessionKey 解析前端真正订阅的路由 session_key。
func (c *Context) ResolveDispatchSessionKey(sessionKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if route, ok := c.sessionRoutes[sessionKey]; ok && route.DispatchSessionKey != "" {
		return route.DispatchSessionKey
	}
	return sessionKey
}

// RequestPermission 发起一个可重放的权限请求。
func (c *Context) RequestPermission(
	ctx context.Context,
	sessionKey string,
	request sdkpermission.Request,
) (sdkpermission.Decision, error) {
	pending := c.newPendingRequest(sessionKey, request)
	c.mu.Lock()
	c.pendingRequests[pending.RequestID] = pending
	c.mu.Unlock()

	go c.dispatchPendingRequest(pending)

	timeout := c.requestTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case decision := <-pending.ResponseCh:
		c.finalizeRequest(pending, "answered")
		return decision, nil
	case <-ctx.Done():
		c.finalizeRequest(pending, "cancelled")
		return sdkpermission.Deny("Permission request cancelled", request.ToolName == "AskUserQuestion"), nil
	case <-timer.C:
		c.finalizeRequest(pending, "expired")
		return sdkpermission.Deny("Permission request timeout", request.ToolName == "AskUserQuestion"), nil
	}
}

// HandlePermissionResponse 处理前端提交的权限决策。
func (c *Context) HandlePermissionResponse(message map[string]any) bool {
	requestID := normalizeString(message["request_id"])
	if requestID == "" {
		return false
	}

	c.mu.RLock()
	pending := c.pendingRequests[requestID]
	c.mu.RUnlock()
	if pending == nil {
		return false
	}

	decision := c.buildPermissionDecision(pending, message)
	select {
	case pending.ResponseCh <- decision:
		c.finalizeRequest(pending, "answered")
	default:
	}
	return true
}

// CancelRequestsForSession 取消指定运行时 session 下的待确认权限请求。
func (c *Context) CancelRequestsForSession(sessionKey string, message string) int {
	if sessionKey == "" {
		return 0
	}

	c.mu.RLock()
	requests := make([]*PendingRequest, 0)
	for _, pending := range c.pendingRequests {
		if pending.SessionKey == sessionKey {
			requests = append(requests, pending)
		}
	}
	c.mu.RUnlock()

	for _, pending := range requests {
		select {
		case pending.ResponseCh <- sdkpermission.Deny(message, true):
			c.finalizeRequest(pending, "cancelled")
		default:
		}
	}
	return len(requests)
}

func (c *Context) pruneClosedBindingsLocked(sessionKey string) {
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		delete(c.sessionBindings, sessionKey)
		return
	}

	for senderKey, binding := range bindings {
		if binding.Sender == nil || binding.Sender.IsClosed() {
			c.removeBindingLocked(sessionKey, senderKey)
		}
	}

	bindings = c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		delete(c.sessionBindings, sessionKey)
	}
}

func (c *Context) removeBindingLocked(sessionKey string, senderKey string) {
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		return
	}

	delete(bindings, senderKey)
	if len(bindings) == 0 {
		delete(c.sessionBindings, sessionKey)
	}

	sessions := c.senderSessions[senderKey]
	if len(sessions) > 0 {
		delete(sessions, sessionKey)
		if len(sessions) == 0 {
			delete(c.senderSessions, senderKey)
		}
	}
}
