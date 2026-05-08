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
	agents   agentWorkspaceResolver
	channels map[string]*registeredChannel
	ingress  IngressAcceptor
	running  bool
	runCtx   context.Context
	logger   *slog.Logger
}

type registeredChannel struct {
	ownerUserID string
	channelType string
	channel     DeliveryChannel
	started     bool
	lastError   string
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
		agents:   agents,
		channels: make(map[string]*registeredChannel),
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
	r.RegisterForOwner("", channel)
}

// RegisterForOwner 按 owner 注册投递通道；同一 owner 的同类通道会替换旧实例。
func (r *Router) RegisterForOwner(ownerUserID string, channel DeliveryChannel) {
	if channel == nil {
		return
	}
	entry := r.newRegisteredChannel(ownerUserID, channel)
	r.mu.Lock()
	replaced := r.channels[channelRouteKey(entry.ownerUserID, entry.channelType)]
	r.channels[channelRouteKey(entry.ownerUserID, entry.channelType)] = entry
	r.mu.Unlock()
	if replaced != nil && replaced.channel != nil && replaced.channel != channel {
		_ = replaced.channel.Stop(context.Background())
	}
}

// RegisterAndStart 注册通道；如果路由器已经启动，则立即启动该通道。
func (r *Router) RegisterAndStart(ctx context.Context, channel DeliveryChannel) error {
	return r.RegisterAndStartForOwner(ctx, "", channel)
}

// RegisterAndStartForOwner 按 owner 注册通道；如果路由器已经启动，则立即启动该通道。
func (r *Router) RegisterAndStartForOwner(ctx context.Context, ownerUserID string, channel DeliveryChannel) error {
	if channel == nil {
		return nil
	}
	entry := r.newRegisteredChannel(ownerUserID, channel)

	r.mu.Lock()
	replaced := r.channels[channelRouteKey(entry.ownerUserID, entry.channelType)]
	r.channels[channelRouteKey(entry.ownerUserID, entry.channelType)] = entry
	running := r.running
	runCtx := r.runCtx
	r.mu.Unlock()
	if replaced != nil && replaced.channel != nil && replaced.channel != channel {
		_ = replaced.channel.Stop(context.Background())
	}

	if !running {
		return nil
	}
	if runCtx == nil {
		runCtx = ctx
	}
	if err := channel.Start(runCtx); err != nil {
		r.markChannelStartResult(entry.ownerUserID, entry.channelType, false, err)
		return err
	}
	r.markChannelStartResult(entry.ownerUserID, entry.channelType, true, nil)
	return nil
}

// UnregisterForOwner 停止并移除指定 owner 的通道实例。
func (r *Router) UnregisterForOwner(ctx context.Context, ownerUserID string, channelType string) {
	key := channelRouteKey(normalizeChannelOwnerUserID(ownerUserID), normalizeChannelType(channelType))
	r.mu.Lock()
	entry := r.channels[key]
	delete(r.channels, key)
	r.mu.Unlock()
	if entry != nil && entry.channel != nil {
		_ = entry.channel.Stop(ctx)
	}
}

func (r *Router) newRegisteredChannel(ownerUserID string, channel DeliveryChannel) *registeredChannel {
	if aware, ok := channel.(ingressAwareChannel); ok {
		aware.SetIngress(r.ingress)
	}
	channelType := normalizeChannelType(channel.ChannelType())
	return &registeredChannel{
		ownerUserID: normalizeChannelOwnerUserID(ownerUserID),
		channelType: channelType,
		channel:     channel,
		started:     isAlwaysReadyChannel(channelType),
	}
}

func (r *Router) markChannelStartResult(ownerUserID string, channelType string, started bool, startErr error) {
	key := channelRouteKey(normalizeChannelOwnerUserID(ownerUserID), normalizeChannelType(channelType))
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.channels[key]
	if entry == nil {
		return
	}
	entry.started = started || isAlwaysReadyChannel(entry.channelType)
	if startErr != nil {
		entry.lastError = startErr.Error()
		return
	}
	entry.lastError = ""
}

func channelRouteKey(ownerUserID string, channelType string) string {
	return normalizeChannelOwnerUserID(ownerUserID) + "/" + normalizeChannelType(channelType)
}

func isAlwaysReadyChannel(channelType string) bool {
	switch normalizeChannelType(channelType) {
	case ChannelTypeWebSocket, ChannelTypeInternal:
		return true
	default:
		return false
	}
}

func (r *Router) resolveDeliveryOwner(ctx context.Context, agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || r.agents == nil {
		return normalizeChannelOwnerUserID("")
	}
	agentValue, err := r.agents.GetAgent(ctx, agentID)
	if err != nil || agentValue == nil {
		return normalizeChannelOwnerUserID("")
	}
	return normalizeChannelOwnerUserID(agentValue.OwnerUserID)
}

func (r *Router) channelForDelivery(ctx context.Context, agentID string, channelType string) DeliveryChannel {
	channelType = normalizeChannelType(channelType)
	ownerUserID := r.resolveDeliveryOwner(ctx, agentID)
	if channel := r.readyChannelForOwner(ownerUserID, channelType); channel != nil {
		return channel
	}
	if ownerUserID != normalizeChannelOwnerUserID("") {
		return r.readyChannelForOwner("", channelType)
	}
	return nil
}

func (r *Router) readyChannelForOwner(ownerUserID string, channelType string) DeliveryChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry := r.channels[channelRouteKey(normalizeChannelOwnerUserID(ownerUserID), normalizeChannelType(channelType))]
	if entry == nil || !entry.started {
		return nil
	}
	return entry.channel
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

// Get 返回指定通道。
func (r *Router) Get(channelType string) DeliveryChannel {
	return r.GetForOwner("", channelType)
}

// GetForOwner 返回指定 owner 的指定通道实例，不代表该实例已经启动成功。
func (r *Router) GetForOwner(ownerUserID string, channelType string) DeliveryChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry := r.channels[channelRouteKey(normalizeChannelOwnerUserID(ownerUserID), normalizeChannelType(channelType))]
	if entry == nil {
		return nil
	}
	return entry.channel
}

// IsReadyForOwner 返回指定 owner 的通道是否已启动成功。
func (r *Router) IsReadyForOwner(ownerUserID string, channelType string) bool {
	return r.readyChannelForOwner(ownerUserID, channelType) != nil
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

	channel := r.channelForDelivery(ctx, agentID, normalized.Channel)
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
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if seen[item.channelType] {
			continue
		}
		seen[item.channelType] = true
		result = append(result, item.channelType)
	}
	return result
}

func (r *Router) snapshotChannels() []registeredChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.channels))
	for key := range r.channels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]registeredChannel, 0, len(keys))
	for _, key := range keys {
		entry := r.channels[key]
		if entry == nil || entry.channel == nil {
			continue
		}
		items = append(items, *entry)
	}
	return items
}
