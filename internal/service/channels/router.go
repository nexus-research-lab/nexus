package channels

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
	deliveryroute "github.com/nexus-research-lab/nexus/internal/service/channels/deliveryroute"
)

// Router 负责管理通道生命周期与统一投递。
type Router struct {
	mu             sync.RWMutex
	deliveryRoutes *deliveryroute.Store
	agents         agentWorkspaceResolver
	channels       map[string]*registeredChannel
	ingress        IngressAcceptor
	running        bool
	runCtx         context.Context
	logger         *slog.Logger
}

type registeredChannel struct {
	ownerUserID string
	channelType string
	channel     DeliveryChannel
	started     bool
	lastError   string
}

type loggerAwareChannel interface {
	SetLogger(*slog.Logger)
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
	defer r.mu.Unlock()
	r.ingress = ingress
	for _, entry := range r.channels {
		if entry == nil || entry.channel == nil {
			continue
		}
		aware, ok := entry.channel.(ingressAwareChannel)
		if !ok {
			continue
		}
		aware.SetIngress(ingress)
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
			r.markChannelStartResult(item.ownerUserID, item.channelType, false, err)
			r.loggerFor(ctx).Error("启动通道失败",
				"owner_user_id", item.ownerUserID,
				"channel", item.channelType,
				"err", err,
			)
			continue
		}
		r.markChannelStartResult(item.ownerUserID, item.channelType, true, nil)
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
		r.markChannelStartResult(items[index].ownerUserID, items[index].channelType, false, nil)
	}
}
