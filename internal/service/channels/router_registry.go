// INPUT: owner+channel 维度的运行实例、Router 级 Session 解析器、热启动与注销请求。
// OUTPUT: 继承统一依赖、每个路由键串行且 generation 单调的注册表状态。
// POS: Router 的实例替换边界，阻止失败或过期候选污染当前路由。
package channels

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
)

// RegisterForOwner 按 owner 注册投递通道；同一 owner 的同类通道会替换旧实例。
func (r *Router) RegisterForOwner(ownerUserID string, channel DeliveryChannel) {
	if channel == nil {
		return
	}
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType := normalizeChannelType(channel.ChannelType())
	unlock := r.lockRouteMutation(ownerUserID, channelType)
	defer unlock()

	entry := r.newRegisteredChannel(ownerUserID, channel)
	key := channelRouteKey(entry.ownerUserID, entry.channelType)
	r.mu.Lock()
	replaced := r.channels[key]
	adopted := replaced != nil &&
		replaced.channel != nil &&
		replaced.channel != channel &&
		adoptReplacedChannel(channel, replaced.channel)
	r.channels[key] = entry
	r.mu.Unlock()
	if replaced != nil && replaced.channel != nil && replaced.channel != channel && !adopted {
		_ = replaced.channel.Stop(context.Background())
	}
}

// RegisterAndStartForOwner 按 owner 注册通道；如果路由器已经启动，则立即启动该通道。
func (r *Router) RegisterAndStartForOwner(ctx context.Context, ownerUserID string, channel DeliveryChannel) error {
	if channel == nil {
		return nil
	}
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType := normalizeChannelType(channel.ChannelType())
	unlock := r.lockRouteMutation(ownerUserID, channelType)
	defer unlock()

	entry := r.newRegisteredChannel(ownerUserID, channel)
	key := channelRouteKey(entry.ownerUserID, entry.channelType)

	r.mu.Lock()
	replaced := r.channels[key]
	running := r.running
	runCtx := r.runCtx
	if !running {
		adopted := replaced != nil &&
			replaced.channel != nil &&
			replaced.channel != channel &&
			adoptReplacedChannel(channel, replaced.channel)
		r.channels[key] = entry
		r.mu.Unlock()
		if replaced != nil && replaced.channel != nil && replaced.channel != channel && !adopted {
			_ = replaced.channel.Stop(context.Background())
		}
		return nil
	}
	r.mu.Unlock()

	if err := channel.Start(runCtx); err != nil {
		_ = channel.Stop(context.Background())
		return err
	}

	r.mu.Lock()
	replaced = r.channels[key]
	if !r.running {
		entry.started = isAlwaysReadyChannel(entry.channelType)
		r.channels[key] = entry
		r.mu.Unlock()
		_ = channel.Stop(context.Background())
		return nil
	}
	adopted := replaced != nil &&
		replaced.channel != nil &&
		replaced.channel != channel &&
		adoptReplacedChannel(channel, replaced.channel)
	entry.started = true
	entry.lastError = ""
	r.channels[key] = entry
	r.mu.Unlock()
	if replaced != nil && replaced.channel != nil && replaced.channel != channel && !adopted {
		_ = replaced.channel.Stop(context.Background())
	}
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
func (r *Router) UnregisterForOwner(ctx context.Context, ownerUserID string, channelType string) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeChannelType(channelType)
	unlock := r.lockRouteMutation(ownerUserID, channelType)
	defer unlock()

	key := channelRouteKey(ownerUserID, channelType)
	r.mu.Lock()
	entry := r.channels[key]
	delete(r.channels, key)
	r.mu.Unlock()
	if entry != nil && entry.channel != nil {
		return entry.channel.Stop(ctx)
	}
	return nil
}

func (r *Router) newRegisteredChannel(ownerUserID string, channel DeliveryChannel) *registeredChannel {
	r.mu.Lock()
	logger := r.logger
	ingress := r.ingress
	sessions := r.sessions
	r.nextGeneration++
	generation := r.nextGeneration
	r.mu.Unlock()

	channelType := normalizeChannelType(channel.ChannelType())
	entry := &registeredChannel{
		ownerUserID: normalizeChannelOwnerUserID(ownerUserID),
		channelType: channelType,
		channel:     channel,
		generation:  generation,
		started:     isAlwaysReadyChannel(channelType),
	}
	setChannelLogger(channel, logger)
	if projector, ok := channel.(*sessionDeliveryChannel); ok {
		projector.sessions = sessions
	}
	if aware, ok := channel.(ingressAwareChannel); ok {
		aware.SetIngress(r.ingressForRegisteredChannel(entry, ingress))
	}
	return entry
}

func (r *Router) markChannelStartResult(candidate *registeredChannel, started bool, startErr error) bool {
	if candidate == nil {
		return false
	}
	key := channelRouteKey(candidate.ownerUserID, candidate.channelType)
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.channels[key]
	if entry == nil || entry.generation != candidate.generation {
		return false
	}
	entry.started = started || isAlwaysReadyChannel(entry.channelType)
	if startErr != nil {
		entry.lastError = startErr.Error()
		return true
	}
	entry.lastError = ""
	return true
}

func (r *Router) lockRouteMutation(ownerUserID string, channelType string) func() {
	key := channelRouteKey(
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeChannelType(channelType),
	)
	value, _ := r.routeLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
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
