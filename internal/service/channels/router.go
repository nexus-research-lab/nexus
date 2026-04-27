package channels

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

// Router 负责管理通道生命周期与统一投递。
type Router struct {
	mu       sync.RWMutex
	memory   *deliveryMemory
	channels map[string]DeliveryChannel
	ingress  IngressAcceptor
	logger   *slog.Logger
}

// NewRouter 创建通道路由器。
func NewRouter(
	cfg config.Config,
	db *sql.DB,
	agents agentWorkspaceResolver,
	permission *permissionctx.Context,
) *Router {
	router := &Router{
		memory:   newDeliveryMemory(cfg, db),
		channels: make(map[string]DeliveryChannel),
		logger:   logx.NewDiscardLogger(),
	}
	router.Register(newSessionDeliveryChannel(ChannelTypeWebSocket, agents, permission, cfg.WorkspacePath))
	router.Register(newSessionDeliveryChannel(ChannelTypeInternal, agents, permission, cfg.WorkspacePath))
	if cfg.DiscordEnabled && strings.TrimSpace(cfg.DiscordBotToken) != "" {
		router.Register(newDiscordChannel(cfg.DiscordBotToken, nil))
	}
	if cfg.TelegramEnabled && strings.TrimSpace(cfg.TelegramBotToken) != "" {
		router.Register(newTelegramChannel(cfg.TelegramBotToken, nil))
	}
	return router
}

// SetLogger 注入业务日志实例。
func (r *Router) SetLogger(logger *slog.Logger) {
	if logger == nil {
		r.logger = logx.NewDiscardLogger()
		return
	}
	r.logger = logger
}

// Register 注册一个投递通道。
func (r *Router) Register(channel DeliveryChannel) {
	if channel == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if aware, ok := channel.(ingressAwareChannel); ok {
		aware.SetIngress(r.ingress)
	}
	r.channels[normalizeChannelType(channel.ChannelType())] = channel
}

// SetIngress 为支持真实入口的通道注入统一 ingress 处理器。
func (r *Router) SetIngress(ingress IngressAcceptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ingress = ingress
	for _, channel := range r.channels {
		aware, ok := channel.(ingressAwareChannel)
		if !ok {
			continue
		}
		aware.SetIngress(ingress)
	}
}

// Start 启动全部通道。
func (r *Router) Start(ctx context.Context) error {
	for _, item := range r.snapshotChannels() {
		r.loggerFor(ctx).Info("启动通道",
			"channel", item.ChannelType(),
		)
		if err := item.Start(ctx); err != nil {
			r.loggerFor(ctx).Error("启动通道失败",
				"channel", item.ChannelType(),
				"err", err,
			)
			return err
		}
	}
	return nil
}

// Stop 停止全部通道。
func (r *Router) Stop(ctx context.Context) {
	items := r.snapshotChannels()
	for index := len(items) - 1; index >= 0; index-- {
		r.loggerFor(ctx).Info("停止通道",
			"channel", items[index].ChannelType(),
		)
		_ = items[index].Stop(ctx)
	}
}

// Get 返回指定通道。
func (r *Router) Get(channelType string) DeliveryChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels[normalizeChannelType(channelType)]
}

// GetLastRoute 读取最近一次成功目标。
func (r *Router) GetLastRoute(ctx context.Context, agentID string) (*DeliveryTarget, error) {
	if r.memory == nil {
		return nil, nil
	}
	return r.memory.GetLastRoute(ctx, agentID)
}

// RememberRoute 记录一条可复用的显式路由。
func (r *Router) RememberRoute(ctx context.Context, agentID string, target DeliveryTarget) (*DeliveryTarget, error) {
	if r.memory == nil {
		return nil, nil
	}
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone || normalized.Mode == DeliveryModeLast {
		normalized.Mode = DeliveryModeExplicit
	}
	remembered, err := r.memory.RememberRoute(ctx, strings.TrimSpace(agentID), normalized)
	if err != nil {
		r.loggerFor(ctx).Error("记录最近投递目标失败",
			"agent_id", strings.TrimSpace(agentID),
			"channel", normalized.Channel,
			"err", err,
		)
		return nil, err
	}
	r.loggerFor(ctx).Info("记录最近投递目标",
		"agent_id", strings.TrimSpace(agentID),
		"channel", normalized.Channel,
		"mode", normalized.Mode,
	)
	return remembered, nil
}

// RememberWebSocketRoute 把当前浏览器会话注册成最近目标。
func (r *Router) RememberWebSocketRoute(ctx context.Context, sessionKey string) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent || strings.TrimSpace(parsed.AgentID) == "" {
		return nil
	}
	_, err := r.RememberRoute(ctx, parsed.AgentID, DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeWebSocket,
		To:         strings.TrimSpace(sessionKey),
		ThreadID:   parsed.ThreadID,
		SessionKey: strings.TrimSpace(sessionKey),
	})
	return err
}

// DeliverText 按目标模式解析并完成文本投递。
func (r *Router) DeliverText(ctx context.Context, agentID string, text string, target DeliveryTarget) (DeliveryTarget, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(text) == "" || normalized.Mode == DeliveryModeNone {
		return normalized, nil
	}
	if normalized.Mode == DeliveryModeLast {
		lastTarget, err := r.GetLastRoute(ctx, agentID)
		if err != nil {
			r.loggerFor(ctx).Error("读取最近投递目标失败",
				"agent_id", agentID,
				"err", err,
			)
			return DeliveryTarget{}, err
		}
		if lastTarget == nil {
			err = fmt.Errorf("last delivery target is not available for agent: %s", strings.TrimSpace(agentID))
			r.loggerFor(ctx).Warn("最近投递目标不存在",
				"agent_id", agentID,
				"err", err,
			)
			return DeliveryTarget{}, err
		}
		normalized = lastTarget.Normalized()
	}
	if err := normalized.Validate(); err != nil {
		return DeliveryTarget{}, err
	}

	channel := r.Get(normalized.Channel)
	if channel == nil {
		err := fmt.Errorf("delivery sender is not configured for channel: %s", normalized.Channel)
		r.loggerFor(ctx).Error("投递通道未配置",
			"agent_id", agentID,
			"channel", normalized.Channel,
			"err", err,
		)
		return DeliveryTarget{}, err
	}
	if err := channel.SendDeliveryText(ctx, normalized, text); err != nil {
		r.loggerFor(ctx).Error("文本投递失败",
			"agent_id", agentID,
			"channel", normalized.Channel,
			"to", normalized.To,
			"thread_id", normalized.ThreadID,
			"err", err,
		)
		return DeliveryTarget{}, err
	}
	if strings.TrimSpace(agentID) != "" {
		if _, err := r.RememberRoute(ctx, agentID, normalized); err != nil {
			return DeliveryTarget{}, err
		}
	}
	r.loggerFor(ctx).Info("文本投递成功",
		"agent_id", agentID,
		"channel", normalized.Channel,
		"to", normalized.To,
		"thread_id", normalized.ThreadID,
		"chars", len([]rune(strings.TrimSpace(text))),
	)
	return normalized, nil
}

func (r *Router) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, r.logger)
}

// RegisteredChannelTypes 返回当前已注册的通道类型快照。
func (r *Router) RegisteredChannelTypes() []string {
	items := r.snapshotChannels()
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ChannelType())
	}
	return result
}

func (r *Router) snapshotChannels() []DeliveryChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.channels))
	for key := range r.channels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]DeliveryChannel, 0, len(keys))
	for _, key := range keys {
		items = append(items, r.channels[key])
	}
	return items
}
