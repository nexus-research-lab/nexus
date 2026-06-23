package channels

import (
	"context"
	"maps"
	"slices"
	"strings"
)

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
	if replaced != nil && replaced.channel != nil && replaced.channel != channel && !adoptReplacedChannel(channel, replaced.channel) {
		_ = replaced.channel.Stop(context.Background())
	}
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
	if replaced != nil && replaced.channel != nil && replaced.channel != channel && !adoptReplacedChannel(channel, replaced.channel) {
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

type replacementAdoptingChannel interface {
	AdoptReplacedChannel(DeliveryChannel) bool
}

func adoptReplacedChannel(channel DeliveryChannel, replaced DeliveryChannel) bool {
	if channel == nil || replaced == nil || channel == replaced {
		return false
	}
	adopter, ok := channel.(replacementAdoptingChannel)
	if !ok {
		return false
	}
	return adopter.AdoptReplacedChannel(replaced)
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
	r.mu.RLock()
	logger := r.logger
	ingress := r.ingress
	r.mu.RUnlock()

	setChannelLogger(channel, logger)
	if aware, ok := channel.(ingressAwareChannel); ok {
		aware.SetIngress(ingress)
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

	keys := slices.Sorted(maps.Keys(r.channels))

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
