// INPUT: WebSocket 请求、共享服务依赖与各 owner/session subscription registry。
// OUTPUT: 认证连接生命周期、协议命令路由，以及 Room/Goal/Execution 实时事件投递。
// POS: HTTP WebSocket transport 入口；不拥有任何业务 mutation 规则。
package websocket

import (
	"context"
	"net/http"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	websocketReadLimit   = 4 << 20
	websocketReadTimeout = 90 * time.Second
	websocketPingEvery   = 30 * time.Second
)

// Handler 封装 WebSocket 生命周期与控制消息分发。
type Handler struct {
	api                    *handlershared.API
	roomService            *roompkg.Service
	roomRealtime           roomRealtimeService
	dm                     *dmsvc.Service
	goals                  *goalsvc.Service
	permission             *permissionctx.Context
	runtime                *runtimectx.Manager
	contextUsage           contextUsageSnapshotProvider
	channels               *channelspkg.Router
	hostCommands           *slashcommandsvc.Registry
	commandCatalog         *slashcommandsvc.Catalog
	runtimeKindResolver    func(context.Context, string) (agentclient.RuntimeKind, error)
	roomSubs               *roomSubscriptionRegistry
	workspaceSubs          *workspaceSubscriptionRegistry
	appEventSubs           *appEventSubscriptionRegistry
	goalRPCSubs            *appServerGoalRPCRegistry
	executionInvalidations *executionInvalidationRegistry
	channelAuthorization   *channelAuthorizationTransport
	allowedOrigins         []string
}

// roomRealtimeService 是 WebSocket 控制面和 Room 订阅恢复实际需要的最小接口。
type roomRealtimeService interface {
	HandleChat(context.Context, roomrealtime.ChatRequest) error
	HandleInterrupt(context.Context, roomrealtime.InterruptRequest) error
	HandleInputQueue(context.Context, roomrealtime.InputQueueRequest) (protocol.InputQueueMutationResult, error)
	InputQueueSnapshotEvent(context.Context, string, string) (protocol.EventMessage, error)
	GetActiveRoundSnapshot(string) *roomrealtime.ActiveRoundSnapshot
	SetRoomBroadcaster(roomrealtime.RoomBroadcaster)
}

// NewHandler 创建 WebSocket handler。
func NewHandler(
	api *handlershared.API,
	roomService *roompkg.Service,
	roomRealtime roomRealtimeService,
	dm *dmsvc.Service,
	goals *goalsvc.Service,
	permission *permissionctx.Context,
	runtime *runtimectx.Manager,
	contextUsage contextUsageSnapshotProvider,
	channels *channelspkg.Router,
	workspaceService *workspacepkg.Service,
	runtimeProvider func(string) RuntimeSnapshot,
	allowedOrigins []string,
	hostCommands *slashcommandsvc.Registry,
	commandCatalog *slashcommandsvc.Catalog,
	runtimeKindResolver func(context.Context, string) (agentclient.RuntimeKind, error),
) *Handler {
	if hostCommands == nil {
		hostCommands = slashcommandsvc.NewRegistry()
	}
	handler := &Handler{
		api:                    api,
		roomService:            roomService,
		roomRealtime:           roomRealtime,
		dm:                     dm,
		goals:                  goals,
		permission:             permission,
		runtime:                runtime,
		contextUsage:           contextUsage,
		channels:               channels,
		hostCommands:           hostCommands,
		commandCatalog:         commandCatalog,
		runtimeKindResolver:    runtimeKindResolver,
		roomSubs:               newRoomSubscriptionRegistry(128),
		workspaceSubs:          newWorkspaceSubscriptionRegistry(workspaceService, runtimeProvider),
		appEventSubs:           newAppEventSubscriptionRegistry(),
		goalRPCSubs:            newAppServerGoalRPCRegistry(),
		executionInvalidations: newExecutionInvalidationRegistry(),
		channelAuthorization:   newChannelAuthorizationTransport(),
		allowedOrigins:         allowedOrigins,
	}
	if roomRealtime != nil {
		roomRealtime.SetRoomBroadcaster(handler.roomSubs)
	}
	if goals != nil {
		goals.SetEventBroadcaster(newGoalEventBroadcaster(permission, handler.goalRPCSubs))
	}
	return handler
}

// contextUsageSnapshotProvider 是历史 Session 重绑定所需的最小持久化读取边界。
type contextUsageSnapshotProvider interface {
	GetPersistedContextUsageSnapshots(
		context.Context,
		string,
	) (map[string]protocol.ContextUsageData, error)
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
	h.ensureChannelAuthorizationTransport().registerAuthenticatedSender(
		request.Context(),
		sender,
	)
	defer func() {
		sender.MarkClosed()
		h.ensureChannelAuthorizationTransport().unregisterSender(sender)
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
		if h.executionInvalidations != nil {
			h.executionInvalidations.UnregisterSender(sender)
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
