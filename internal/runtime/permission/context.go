package permission

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// Sender 抽象出 WebSocket 级别的事件发送能力。
type Sender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

// SessionBinding 表示 session 的控制权快照。
type SessionBinding struct {
	ControllerClientID string
	ObserverCount      int
	BoundClientCount   int
}

type senderBinding struct {
	Sender    Sender
	ClientID  string
	BindOrder int64
}

// Context 保存 session 绑定、控制端与广播逻辑。
type Context struct {
	mu                  sync.RWMutex
	sessionBindings     map[string]map[string]senderBinding
	senderSessions      map[string]map[string]struct{}
	controllerSenderIDs map[string]string
	bindSequence        int64
	sessionRoutes       map[string]RouteContext
	pendingRequests     map[string]*PendingRequest
	requestTimeout      time.Duration
}

// NewContext 创建权限运行时上下文。
func NewContext() *Context {
	return &Context{
		sessionBindings:     make(map[string]map[string]senderBinding),
		senderSessions:      make(map[string]map[string]struct{}),
		controllerSenderIDs: make(map[string]string),
		sessionRoutes:       make(map[string]RouteContext),
		pendingRequests:     make(map[string]*PendingRequest),
		requestTimeout:      time.Minute,
	}
}

// BindSession 绑定 sender 到 session，并按需要抢占控制权。
func (c *Context) BindSession(sessionKey string, sender Sender, clientID string, requestControl bool) SessionBinding {
	if sender == nil || sender.IsClosed() || sessionKey == "" {
		return c.Lookup(sessionKey)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	bindings := c.sessionBindings[sessionKey]
	if bindings == nil {
		bindings = make(map[string]senderBinding)
		c.sessionBindings[sessionKey] = bindings
	}
	c.bindSequence++
	senderKey := sender.Key()
	bindings[senderKey] = senderBinding{
		Sender:    sender,
		ClientID:  normalizeClientID(clientID, senderKey),
		BindOrder: c.bindSequence,
	}

	sessions := c.senderSessions[senderKey]
	if sessions == nil {
		sessions = make(map[string]struct{})
		c.senderSessions[senderKey] = sessions
	}
	sessions[sessionKey] = struct{}{}

	c.pruneClosedBindingsLocked(sessionKey)

	controllerSenderID := c.controllerSenderIDs[sessionKey]
	if requestControl || controllerSenderID == "" || bindings[controllerSenderID].Sender == nil {
		c.controllerSenderIDs[sessionKey] = senderKey
	}
	snapshot := c.lookupLocked(sessionKey)
	go c.replayPendingRequests(sessionKey)
	return snapshot
}

// UnbindSession 解绑 sender 对指定 session 的绑定。
func (c *Context) UnbindSession(sessionKey string, sender Sender) SessionBinding {
	if sender == nil || sessionKey == "" {
		return c.Lookup(sessionKey)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeBindingLocked(sessionKey, sender.Key())
	c.pruneClosedBindingsLocked(sessionKey)
	snapshot := c.lookupLocked(sessionKey)
	go c.replayPendingRequests(sessionKey)
	return snapshot
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
	for _, sessionKey := range changed {
		go c.replayPendingRequests(sessionKey)
	}
	return changed
}

// HasBindings 判断 session 是否存在活跃绑定。
func (c *Context) HasBindings(sessionKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneClosedBindingsLocked(sessionKey)
	return len(c.sessionBindings[sessionKey]) > 0
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

// IsSessionController 判断 sender 是否是当前控制端。
func (c *Context) IsSessionController(sessionKey string, sender Sender) bool {
	if sender == nil {
		return false
	}
	controller := c.ResolveControllerSender(sessionKey)
	return controller != nil && controller.Key() == sender.Key()
}

// ResolveControllerSender 返回当前控制端 sender。
func (c *Context) ResolveControllerSender(sessionKey string) Sender {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneClosedBindingsLocked(sessionKey)
	controllerSenderID := c.controllerSenderIDs[sessionKey]
	if controllerSenderID == "" {
		return nil
	}
	return c.sessionBindings[sessionKey][controllerSenderID].Sender
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

// Lookup 返回当前 session 的控制权快照。
func (c *Context) Lookup(sessionKey string) SessionBinding {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneClosedBindingsLocked(sessionKey)
	return c.lookupLocked(sessionKey)
}

// BroadcastSessionStatus 向当前 session 的全部绑定连接广播 session_status。
func (c *Context) BroadcastSessionStatus(ctx context.Context, sessionKey string, runningRoundIDs []string) []error {
	senders := c.ResolveSessionSenders(sessionKey)
	if len(senders) == 0 {
		return nil
	}
	snapshot := c.Lookup(sessionKey)
	event := protocol.NewEvent(protocol.EventTypeSessionStatus, map[string]any{
		"is_generating":        len(runningRoundIDs) > 0,
		"running_round_ids":    runningRoundIDs,
		"controller_client_id": emptyToNil(snapshot.ControllerClientID),
		"observer_count":       snapshot.ObserverCount,
		"bound_client_count":   snapshot.BoundClientCount,
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
	request sdkprotocol.PermissionRequest,
) (sdkprotocol.PermissionDecision, error) {
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
		return sdkprotocol.DenyPermission("Permission request cancelled", request.ToolName == "AskUserQuestion"), nil
	case <-timer.C:
		c.finalizeRequest(pending, "expired")
		return sdkprotocol.DenyPermission("Permission request timeout", request.ToolName == "AskUserQuestion"), nil
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
		case pending.ResponseCh <- sdkprotocol.DenyPermission(message, true):
			c.finalizeRequest(pending, "cancelled")
		default:
		}
	}
	return len(requests)
}

func (c *Context) lookupLocked(sessionKey string) SessionBinding {
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		return SessionBinding{}
	}

	controllerSenderID := c.controllerSenderIDs[sessionKey]
	controllerClientID := ""
	if controllerBinding, ok := bindings[controllerSenderID]; ok {
		controllerClientID = controllerBinding.ClientID
	}
	boundClientCount := len(bindings)
	return SessionBinding{
		ControllerClientID: controllerClientID,
		ObserverCount:      max(boundClientCount-1, 0),
		BoundClientCount:   boundClientCount,
	}
}

func (c *Context) pruneClosedBindingsLocked(sessionKey string) {
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		delete(c.sessionBindings, sessionKey)
		delete(c.controllerSenderIDs, sessionKey)
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
		delete(c.controllerSenderIDs, sessionKey)
		return
	}

	controllerSenderID := c.controllerSenderIDs[sessionKey]
	if controllerSenderID == "" {
		c.promoteControllerLocked(sessionKey)
		return
	}
	if _, ok := bindings[controllerSenderID]; !ok {
		c.promoteControllerLocked(sessionKey)
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

	if c.controllerSenderIDs[sessionKey] == senderKey {
		delete(c.controllerSenderIDs, sessionKey)
	}
}

func (c *Context) promoteControllerLocked(sessionKey string) {
	bindings := c.sessionBindings[sessionKey]
	if len(bindings) == 0 {
		delete(c.controllerSenderIDs, sessionKey)
		return
	}

	var (
		promotedSenderID string
		maxOrder         int64 = -1
	)
	for senderKey, binding := range bindings {
		if binding.BindOrder > maxOrder {
			maxOrder = binding.BindOrder
			promotedSenderID = senderKey
		}
	}
	if promotedSenderID != "" {
		c.controllerSenderIDs[sessionKey] = promotedSenderID
	}
}

func normalizeClientID(clientID string, senderKey string) string {
	normalized := clientID
	if normalized == "" {
		return "sender:" + senderKey
	}
	return normalized
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func max(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
