// INPUT: owner 隔离的 DeliveryChannel 注册、统一 Session 解析器、生命周期与投递请求。
// OUTPUT: generation 防护的热替换、依赖继承、就绪状态与统一消息投递。
// POS: Channels 运行态路由核心，旧实例的异步完成不得覆盖新实例。
package channels

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
	deliveryroute "github.com/nexus-research-lab/nexus/internal/service/channels/deliveryroute"
)

// Router 负责管理通道生命周期与统一投递。
type Router struct {
	mu              sync.RWMutex
	deliveryRoutes  *deliveryroute.Store
	agents          agentWorkspaceResolver
	sessions        sessionProjectionResolver
	channels        map[string]*registeredChannel
	ingress         IngressAcceptor
	running         bool
	runCtx          context.Context
	logger          *slog.Logger
	routeLocks      sync.Map
	projectionLocks sync.Map
	nextGeneration  uint64
}

type registeredChannel struct {
	ownerUserID string
	channelType string
	channel     DeliveryChannel
	generation  uint64
	started     bool
	lastError   string
}

type loggerAwareChannel interface {
	SetLogger(*slog.Logger)
}

type sessionProjectionResolver interface {
	ResolveDeliverySession(context.Context, string) (*protocol.Session, error)
}

// NewRouter 创建通道路由器。
func NewRouter(
	cfg config.Config,
	db *sql.DB,
	agents agentWorkspaceResolver,
	permission *permissionctx.Context,
) *Router {
	router := &Router{
		deliveryRoutes: deliveryroute.NewStore(cfg, db),
		agents:         agents,
		channels:       make(map[string]*registeredChannel),
		logger:         logx.NewDiscardLogger(),
	}
	router.RegisterForOwner("", newSessionDeliveryChannel(ChannelTypeWebSocket, agents, permission, cfg.WorkspacePath))
	router.RegisterForOwner("", newSessionDeliveryChannel(ChannelTypeInternal, agents, permission, cfg.WorkspacePath))
	if cfg.DiscordEnabled && strings.TrimSpace(cfg.DiscordBotToken) != "" {
		router.RegisterForOwner("", channeladapters.NewDiscordChannel(cfg.DiscordBotToken, nil))
	}
	if cfg.TelegramEnabled && strings.TrimSpace(cfg.TelegramBotToken) != "" {
		router.RegisterForOwner("", channeladapters.NewTelegramChannel(cfg.TelegramBotToken, nil))
	}
	return router
}

// SetLogger 注入业务日志实例。
func (r *Router) SetLogger(logger *slog.Logger) {
	resolved := logger
	if logger == nil {
		resolved = logx.NewDiscardLogger()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = resolved
	for _, entry := range r.channels {
		if entry == nil {
			continue
		}
		setChannelLogger(entry.channel, resolved)
	}
}

// SetSessionProjectionResolver 注入统一 Session 读模型，用于在投递前解析并
// 物化数据库拥有的 Room-backed DM/成员 Session。
func (r *Router) SetSessionProjectionResolver(resolver sessionProjectionResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = resolver
	for _, entry := range r.channels {
		projector, ok := entry.channel.(*sessionDeliveryChannel)
		if ok {
			projector.sessions = resolver
		}
	}
}

func (r *Router) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, r.logger)
}

func setChannelLogger(channel DeliveryChannel, logger *slog.Logger) {
	if channel == nil {
		return
	}
	aware, ok := channel.(loggerAwareChannel)
	if !ok {
		return
	}
	aware.SetLogger(logger)
}

// SetIngress 为支持真实入口的通道注入统一 ingress 处理器。
func (r *Router) SetIngress(ingress IngressAcceptor) {
	r.mu.Lock()
	r.ingress = ingress
	entries := make([]*registeredChannel, 0, len(r.channels))
	for _, entry := range r.channels {
		if entry == nil || entry.channel == nil {
			continue
		}
		entries = append(entries, entry)
	}
	r.mu.Unlock()
	for _, entry := range entries {
		aware, ok := entry.channel.(ingressAwareChannel)
		if !ok {
			continue
		}
		aware.SetIngress(r.ingressForRegisteredChannel(entry, ingress))
	}
}

// Start 启动全部通道。
func (r *Router) Start(ctx context.Context) error {
	r.mu.Lock()
	r.running = true
	r.runCtx = ctx
	r.mu.Unlock()
	for _, item := range r.snapshotChannels() {
		r.loggerFor(ctx).Info("启动通道",
			"owner_user_id", item.ownerUserID,
			"channel", item.channelType,
		)
		if err := item.channel.Start(ctx); err != nil {
			r.markChannelStartResult(&item, false, err)
			_ = item.channel.Stop(ctx)
			r.loggerFor(ctx).Error("启动通道失败",
				"owner_user_id", item.ownerUserID,
				"channel", item.channelType,
				"err", err,
			)
			continue
		}
		if !r.markChannelStartResult(&item, true, nil) {
			_ = item.channel.Stop(ctx)
		}
	}
	return nil
}

// Stop 停止全部通道。
func (r *Router) Stop(ctx context.Context) {
	r.mu.Lock()
	r.running = false
	r.runCtx = nil
	r.mu.Unlock()
	items := r.snapshotChannels()
	for index := len(items) - 1; index >= 0; index-- {
		r.loggerFor(ctx).Info("停止通道",
			"owner_user_id", items[index].ownerUserID,
			"channel", items[index].channelType,
		)
		_ = items[index].channel.Stop(ctx)
		r.markChannelStartResult(&items[index], false, nil)
	}
}
