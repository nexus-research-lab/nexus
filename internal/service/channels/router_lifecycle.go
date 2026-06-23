package channels

import (
	"context"
)

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
