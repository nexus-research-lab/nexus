package websocket

import (
	"context"
	"net/http"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	websocketReadLimit   = 4 << 20
	websocketReadTimeout = 90 * time.Second
	websocketPingEvery   = 30 * time.Second
)

// Handler 封装 WebSocket 生命周期与控制消息分发。
type Handler struct {
	api            *handlershared.API
	roomService    *roompkg.Service
	roomRealtime   *roompkg.RealtimeService
	dm             *dmsvc.Service
	goals          *goalsvc.Service
	permission     *permissionctx.Context
	runtime        *runtimectx.Manager
	channels       *channelspkg.Router
	roomSubs       *roomSubscriptionRegistry
	workspaceSubs  *workspaceSubscriptionRegistry
	appEventSubs   *appEventSubscriptionRegistry
	goalRPCSubs    *appServerGoalRPCRegistry
	allowedOrigins []string
}

// NewHandler 创建 WebSocket handler。
func NewHandler(
	api *handlershared.API,
	roomService *roompkg.Service,
	roomRealtime *roompkg.RealtimeService,
	dm *dmsvc.Service,
	goals *goalsvc.Service,
	permission *permissionctx.Context,
	runtime *runtimectx.Manager,
	channels *channelspkg.Router,
	workspaceService *workspacepkg.Service,
	runtimeProvider func(string) RuntimeSnapshot,
	allowedOrigins []string,
) *Handler {
	handler := &Handler{
		api:            api,
		roomService:    roomService,
		roomRealtime:   roomRealtime,
		dm:             dm,
		goals:          goals,
		permission:     permission,
		runtime:        runtime,
		channels:       channels,
		roomSubs:       newRoomSubscriptionRegistry(128),
		workspaceSubs:  newWorkspaceSubscriptionRegistry(workspaceService, runtimeProvider),
		appEventSubs:   newAppEventSubscriptionRegistry(),
		goalRPCSubs:    newAppServerGoalRPCRegistry(),
		allowedOrigins: allowedOrigins,
	}
	if roomRealtime != nil {
		roomRealtime.SetRoomBroadcaster(handler.roomSubs)
	}
	if goals != nil {
		goals.SetEventBroadcaster(newGoalEventBroadcaster(permission, handler.goalRPCSubs))
	}
	return handler
}

// HandleWebSocket 处理 WebSocket 会话。
func (h *Handler) HandleWebSocket(writer http.ResponseWriter, request *http.Request) {
	originPatterns := h.allowedOrigins
	if len(originPatterns) == 0 {
		// 未配置白名单时保持向后兼容，允许所有来源。
		// 部署环境建议通过 ALLOWED_WEBSOCKET_ORIGINS 显式指定允许的 Origin。
		originPatterns = []string{"*"}
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: originPatterns,
		Subprotocols:   []string{handlershared.DesktopWebSocketSubprotocol},
	})
	if err != nil {
		return
	}
	connection.SetReadLimit(websocketReadLimit)
	sender := handlershared.NewWebSocketSender(connection)
	defer func() {
		sender.MarkClosed()
		if h.workspaceSubs != nil {
			h.workspaceSubs.UnregisterSender(sender)
		}
		if h.roomSubs != nil {
			h.roomSubs.UnregisterSender(sender)
		}
		if h.appEventSubs != nil {
			h.appEventSubs.UnregisterSender(sender)
		}
		if h.goalRPCSubs != nil {
			h.goalRPCSubs.UnregisterSender(sender)
		}
		_ = connection.Close(websocket.StatusNormalClosure, "closed")
		h.broadcastSessionStatus(request.Context(), h.permission.UnregisterSender(sender)...)
	}()

	ctx := request.Context()
	controlDispatcher := newControlMessageDispatcher(ctx)
	defer controlDispatcher.close()
	go h.keepWebSocketAlive(ctx, connection, sender)
	for {
		var inbound map[string]any
		readCtx, cancel := context.WithTimeout(ctx, websocketReadTimeout)
		err := wsjson.Read(readCtx, connection, &inbound)
		cancel()
		if err != nil {
			return
		}
		h.dispatchWebSocketMessageWithControlDispatcher(
			ctx,
			sender,
			inbound,
			controlDispatcher,
		)
	}
}
