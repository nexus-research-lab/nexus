// INPUT: Session 事件 sender、runtime session 路由 lease 与阻塞式人工交互命令。
// OUTPUT: sender/session 绑定、仅由当前 owner 释放的路由映射、DM 生命周期 Room 投影、重连 route 快照，以及请求广播与响应收口。
// POS: runtime permission 的并发上下文与连接生命周期真相源。
package permission

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// Sender 抽象出 session 级别的事件发送能力。
type Sender interface {
	Key() string
	IsClosed() bool
	SendEvent(context.Context, protocol.EventMessage) error
}

// RoomBroadcaster 把带 Room 路由的人工交互事件投影给 Room 订阅者。
type RoomBroadcaster interface {
	Broadcast(context.Context, string, protocol.EventMessage) []error
}

type senderBinding struct {
	Sender Sender
}

type sessionRouteBinding struct {
	leaseID uint64
	route   RouteContext
}

// SessionRouteLease 标识一次 runtime session 路由绑定的所有权。
// 字段保持私有，调用方只能释放 BindSessionRoute 返回的 lease。
type SessionRouteLease struct {
	leaseID    uint64
	sessionKey string
}

// SessionActivityRouteSnapshot exposes the exact runtime Session route needed
// to rebuild one Room's transient execution activity after a WebSocket reconnect.
type SessionActivityRouteSnapshot struct {
	SessionKey string
	Route      RouteContext
}

// Context 保存 session 绑定与权限请求广播逻辑。
type Context struct {
	mu               sync.RWMutex
	sessionBindings  map[string]map[string]senderBinding
	senderSessions   map[string]map[string]struct{}
	sessionRoutes    map[string]sessionRouteBinding
	nextRouteLease   uint64
	pendingRequests  map[string]*PendingRequest
	requestTimeout   time.Duration
	approvalRecorder HumanToolApprovalRecorder
	pendingChanged   chan struct{}
	roomBroadcaster  RoomBroadcaster
}

// SetHumanToolApprovalRecorder 注入高风险业务写入的一次性人工批准记录器。
func (c *Context) SetHumanToolApprovalRecorder(recorder HumanToolApprovalRecorder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.approvalRecorder = recorder
}

// NewContext 创建权限运行时上下文。
func NewContext() *Context {
	return &Context{
		sessionBindings: make(map[string]map[string]senderBinding),
		senderSessions:  make(map[string]map[string]struct{}),
		sessionRoutes:   make(map[string]sessionRouteBinding),
		pendingRequests: make(map[string]*PendingRequest),
		requestTimeout:  time.Minute,
		pendingChanged:  make(chan struct{}),
	}
}

// SetRoomBroadcaster 注入 Room 事件广播器，让侧栏等房间级订阅者共享人工交互状态。
func (c *Context) SetRoomBroadcaster(broadcaster RoomBroadcaster) {
	c.mu.Lock()
	c.roomBroadcaster = broadcaster
	c.mu.Unlock()
}

// PendingRequestState 返回 runtime session 当前是否等待人工交互，以及下一次状态变化信号。
// 调用方收到 changed 后必须重新读取；Context 会为每次变化轮换 channel。
func (c *Context) PendingRequestState(sessionKey string) (pending bool, changed <-chan struct{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, request := range c.pendingRequests {
		if request.SessionKey == sessionKey {
			return true, c.pendingChanged
		}
	}
	return false, c.pendingChanged
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

	changed := slices.Sorted(maps.Keys(sessions))
	for _, sessionKey := range changed {
		c.removeBindingLocked(sessionKey, senderKey)
		c.pruneClosedBindingsLocked(sessionKey)
	}
	delete(c.senderSessions, senderKey)
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
	slices.SortFunc(result, func(left Sender, right Sender) int {
		return cmp.Compare(left.Key(), right.Key())
	})
	return result
}

// BroadcastSessionStatus 向当前 session 的全部绑定连接广播 session_status。
func (c *Context) BroadcastSessionStatus(ctx context.Context, sessionKey string, runningRoundIDs []string) []error {
	senders := c.ResolveSessionSenders(sessionKey)
	event := protocol.NewEvent(protocol.EventTypeSessionStatus, map[string]any{
		"is_generating":     len(runningRoundIDs) > 0,
		"running_round_ids": runningRoundIDs,
	})
	event.SessionKey = sessionKey
	event, _, _ = c.prepareRoutedEvent(sessionKey, event)

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
	if event.SessionKey == "" {
		event.SessionKey = sessionKey
	}
	event, route, roomBroadcaster := c.prepareRoutedEvent(sessionKey, event)
	errs := make([]error, 0)
	for _, sender := range senders {
		if err := sender.SendEvent(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	// canonical DM 的 round 生命周期同时投影到所属聊天容器。侧栏因此不依赖
	// 当前页面是否仍 bind_session；durable 副本还会封住快照到订阅之间的竞态。
	if event.EventType == protocol.EventTypeRoundStatus &&
		roomBroadcaster != nil && route.RoomID != "" && route.ConversationID != "" {
		roomEvent := event
		roomEvent.DeliveryMode = protocol.DeliveryModeDurable
		errs = append(errs, roomBroadcaster.Broadcast(ctx, route.RoomID, roomEvent)...)
	}
	return errs
}

func (c *Context) prepareRoutedEvent(
	sessionKey string,
	event protocol.EventMessage,
) (protocol.EventMessage, RouteContext, RoomBroadcaster) {
	c.mu.RLock()
	binding := c.sessionRoutes[sessionKey]
	roomBroadcaster := c.roomBroadcaster
	c.mu.RUnlock()
	route := binding.route
	if event.RoomID == "" {
		event.RoomID = route.RoomID
	}
	if event.ConversationID == "" {
		event.ConversationID = route.ConversationID
	}
	if event.AgentID == "" {
		event.AgentID = route.AgentID
	}
	return event, route, roomBroadcaster
}

// BindSessionRoute 记录运行时 session 到前端路由 session 的映射，并返回当前绑定的 owner lease。
func (c *Context) BindSessionRoute(sessionKey string, route RouteContext) SessionRouteLease {
	if sessionKey == "" {
		return SessionRouteLease{}
	}
	if route.DispatchSessionKey == "" {
		route.DispatchSessionKey = sessionKey
	}
	c.mu.Lock()
	c.nextRouteLease++
	lease := SessionRouteLease{
		leaseID:    c.nextRouteLease,
		sessionKey: sessionKey,
	}
	c.sessionRoutes[sessionKey] = sessionRouteBinding{
		leaseID: lease.leaseID,
		route:   route,
	}
	c.mu.Unlock()
	return lease
}

// SessionActivityRoutesForRoom returns a stable copy of every runtime Session
// route associated with one chat container. Callers must still ask runtime for
// the current running rounds; historical route bindings alone are not activity.
func (c *Context) SessionActivityRoutesForRoom(roomID string) []SessionActivityRouteSnapshot {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return []SessionActivityRouteSnapshot{}
	}
	c.mu.RLock()
	result := make([]SessionActivityRouteSnapshot, 0)
	for sessionKey, binding := range c.sessionRoutes {
		if strings.TrimSpace(binding.route.RoomID) != roomID {
			continue
		}
		result = append(result, SessionActivityRouteSnapshot{
			SessionKey: sessionKey,
			Route:      binding.route,
		})
	}
	c.mu.RUnlock()
	slices.SortFunc(result, func(left SessionActivityRouteSnapshot, right SessionActivityRouteSnapshot) int {
		return cmp.Compare(left.SessionKey, right.SessionKey)
	})
	return result
}

// UnbindSessionRoute 仅在 lease 仍拥有当前绑定时移除 runtime session 路由。
func (c *Context) UnbindSessionRoute(lease SessionRouteLease) {
	if lease.sessionKey == "" || lease.leaseID == 0 {
		return
	}
	c.mu.Lock()
	binding, ok := c.sessionRoutes[lease.sessionKey]
	if ok && binding.leaseID == lease.leaseID {
		delete(c.sessionRoutes, lease.sessionKey)
	}
	c.mu.Unlock()
}

// ResolveDispatchSessionKey 解析前端真正订阅的路由 session_key。
func (c *Context) ResolveDispatchSessionKey(sessionKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if binding, ok := c.sessionRoutes[sessionKey]; ok && binding.route.DispatchSessionKey != "" {
		return binding.route.DispatchSessionKey
	}
	return sessionKey
}

// RequestPermission 发起一个可重放的阻塞式人工交互请求。
func (c *Context) RequestPermission(
	ctx context.Context,
	sessionKey string,
	request sdkpermission.Request,
) (sdkpermission.Decision, error) {
	decision, _, err := c.RequestPermissionWithID(ctx, sessionKey, request)
	return decision, err
}

// RequestPermissionWithID 与 RequestPermission 相同，并返回宿主生成的精确请求 ID，
// 供高风险领域命令把真人批准关联到 durable audit。
func (c *Context) RequestPermissionWithID(
	ctx context.Context,
	sessionKey string,
	request sdkpermission.Request,
) (sdkpermission.Decision, string, error) {
	pending := c.newPendingRequest(sessionKey, request)
	c.mu.Lock()
	c.pendingRequests[pending.RequestID] = pending
	c.notifyPendingRequestsChangedLocked()
	c.mu.Unlock()

	go c.dispatchPendingRequest(pending)

	select {
	case decision := <-pending.ResponseCh:
		c.finalizeRequest(pending, "answered")
		return decision, pending.RequestID, nil
	case <-ctx.Done():
		c.finalizeRequest(pending, "cancelled")
		return sdkpermission.Deny("Permission request cancelled", request.ToolName == "AskUserQuestion"), pending.RequestID, nil
	}
}

func (c *Context) notifyPendingRequestsChangedLocked() {
	close(c.pendingChanged)
	c.pendingChanged = make(chan struct{})
}

// HandlePermissionResponse 处理前端提交的权限决策。
func (c *Context) HandlePermissionResponse(
	ctx context.Context,
	sessionKey string,
	message map[string]any,
) bool {
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
	if pending.DispatchSessionKey != sessionKey {
		return false
	}

	decision := c.buildPermissionDecision(ctx, pending, message)
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
	delete(bindings, senderKey)
	if len(bindings) == 0 {
		delete(c.sessionBindings, sessionKey)
	}

	sessions := c.senderSessions[senderKey]
	delete(sessions, sessionKey)
	if len(sessions) == 0 {
		delete(c.senderSessions, senderKey)
	}
}
