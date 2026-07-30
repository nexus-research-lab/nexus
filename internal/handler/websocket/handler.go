package websocket

import (
	"context"
	"net/http"
	"sync"
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
)

const (
	websocketReadLimit   = 4 << 20
	websocketReadTimeout = 90 * time.Second
	websocketPingEvery   = 30 * time.Second
)

// Handler 封装 WebSocket 生命周期与控制消息分发。
type Handler struct {
	api                  *handlershared.API
	roomService          *roompkg.Service
	roomRealtime         roomRealtimeService
	dm                   *dmsvc.Service
	goals                *goalsvc.Service
	permission           *permissionctx.Context
	runtime              *runtimectx.Manager
	channels             *channelspkg.Router
	hostCommands         *slashcommandsvc.Registry
	commandCatalogCtx    context.Context
	commandCatalogCancel context.CancelFunc
	commandCatalogMu     sync.Mutex
	commandCatalogWG     sync.WaitGroup
	commandCatalogClosed bool
	roomSubs             *roomSubscriptionRegistry
	workspaceSubs        *workspaceSubscriptionRegistry
	appEventSubs         *appEventSubscriptionRegistry
	goalRPCSubs          *appServerGoalRPCRegistry
	allowedOrigins       []string
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
	channels *channelspkg.Router,
	workspaceService *workspacepkg.Service,
	runtimeProvider func(string) RuntimeSnapshot,
	allowedOrigins []string,
	hostCommands *slashcommandsvc.Registry,
) *Handler {
	if hostCommands == nil {
		hostCommands = slashcommandsvc.NewRegistry()
	}
	commandCatalogCtx, commandCatalogCancel := context.WithCancel(context.Background())
	handler := &Handler{
		api:                  api,
		roomService:          roomService,
		roomRealtime:         roomRealtime,
		dm:                   dm,
		goals:                goals,
		permission:           permission,
		runtime:              runtime,
		channels:             channels,
		hostCommands:         hostCommands,
		commandCatalogCtx:    commandCatalogCtx,
		commandCatalogCancel: commandCatalogCancel,
		roomSubs:             newRoomSubscriptionRegistry(128),
		workspaceSubs:        newWorkspaceSubscriptionRegistry(workspaceService, runtimeProvider),
		appEventSubs:         newAppEventSubscriptionRegistry(),
		goalRPCSubs:          newAppServerGoalRPCRegistry(),
		allowedOrigins:       allowedOrigins,
	}
	if roomRealtime != nil {
		roomRealtime.SetRoomBroadcaster(handler.roomSubs)
	}
	if goals != nil {
		goals.SetEventBroadcaster(newGoalEventBroadcaster(permission, handler.goalRPCSubs))
	}
	return handler
}

// Close 停止 bind_session 启动的后台 runtime 目录同步，并等待其退出。
func (h *Handler) Close() {
	if h == nil {
		return
	}
	h.commandCatalogMu.Lock()
	if h.commandCatalogClosed {
		h.commandCatalogMu.Unlock()
		return
	}
	h.commandCatalogClosed = true
	if h.commandCatalogCancel != nil {
		h.commandCatalogCancel()
	}
	h.commandCatalogMu.Unlock()
	h.commandCatalogWG.Wait()
}

func (h *Handler) startBoundCommandCatalog(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
	parsed protocol.SessionKey,
) {
	if h == nil {
		return
	}
	h.commandCatalogMu.Lock()
	if h.commandCatalogClosed {
		h.commandCatalogMu.Unlock()
		return
	}
	h.commandCatalogWG.Add(1)
	lifecycleCtx := h.commandCatalogCtx
	h.commandCatalogMu.Unlock()

	jobCtx, cancel := context.WithCancel(ctx)
	stopLifecycle := func() {}
	if lifecycleCtx != nil {
		stop := context.AfterFunc(lifecycleCtx, cancel)
		stopLifecycle = func() {
			_ = stop()
		}
	}
	go func() {
		defer h.commandCatalogWG.Done()
		defer stopLifecycle()
		defer cancel()
		h.initializeBoundCommandCatalog(jobCtx, sender, sessionKey, parsed)
	}()
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
