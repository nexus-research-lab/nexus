package channels

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetLastRoute 读取最近一次成功目标。
func (r *Router) GetLastRoute(ctx context.Context, agentID string) (*DeliveryTarget, error) {
	if r.deliveryRoutes == nil {
		return nil, nil
	}
	return r.deliveryRoutes.GetLastRoute(ctx, agentID)
}

// GetSessionRoute 读取指定 session 最近一次成功目标。
func (r *Router) GetSessionRoute(ctx context.Context, agentID string, sessionKey string) (*DeliveryTarget, error) {
	if r.deliveryRoutes == nil {
		return nil, nil
	}
	agentID = strings.TrimSpace(agentID)
	sessionKey = strings.TrimSpace(sessionKey)
	return r.deliveryRoutes.GetSessionRoute(ctx, agentID, sessionKey)
}

// RememberRoute 记录一条可复用的显式路由。
func (r *Router) RememberRoute(ctx context.Context, agentID string, target DeliveryTarget) (*DeliveryTarget, error) {
	if r.deliveryRoutes == nil {
		return nil, nil
	}
	agentID = strings.TrimSpace(agentID)
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone || normalized.Mode == DeliveryModeLast {
		normalized.Mode = DeliveryModeExplicit
	}
	remembered, err := r.deliveryRoutes.RememberRoute(ctx, agentID, normalized)
	if err != nil {
		r.loggerFor(ctx).Error("记录最近投递目标失败",
			"agent_id", agentID,
			"channel", normalized.Channel,
			"err", err,
		)
		return nil, err
	}
	r.loggerFor(ctx).Debug("记录最近投递目标",
		"agent_id", agentID,
		"channel", normalized.Channel,
		"mode", normalized.Mode,
	)
	return remembered, nil
}

// RememberSessionRoute 记录指定 session 的显式路由。
func (r *Router) RememberSessionRoute(ctx context.Context, agentID string, sessionKey string, target DeliveryTarget) (*DeliveryTarget, error) {
	if r.deliveryRoutes == nil {
		return nil, nil
	}
	agentID = strings.TrimSpace(agentID)
	sessionKey = strings.TrimSpace(sessionKey)
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone || normalized.Mode == DeliveryModeLast {
		normalized.Mode = DeliveryModeExplicit
	}
	remembered, err := r.deliveryRoutes.RememberSessionRoute(ctx, agentID, sessionKey, normalized)
	if err != nil {
		r.loggerFor(ctx).Error("记录 session 投递目标失败",
			"agent_id", agentID,
			"session_key", sessionKey,
			"channel", normalized.Channel,
			"err", err,
		)
		return nil, err
	}
	r.loggerFor(ctx).Debug("记录 session 投递目标",
		"agent_id", agentID,
		"session_key", sessionKey,
		"channel", normalized.Channel,
		"mode", normalized.Mode,
	)
	return remembered, nil
}

// RememberWebSocketRoute 把当前浏览器会话注册成最近目标。
func (r *Router) RememberWebSocketRoute(ctx context.Context, sessionKey string) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.AgentID == "" ||
		protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment {
		return nil
	}
	sessionKey = parsed.Raw
	_, err := r.RememberRoute(ctx, parsed.AgentID, DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeWebSocket,
		To:         sessionKey,
		ThreadID:   parsed.ThreadID,
		SessionKey: sessionKey,
	})
	if err != nil {
		return err
	}
	_, err = r.RememberSessionRoute(ctx, parsed.AgentID, sessionKey, DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeWebSocket,
		To:         sessionKey,
		ThreadID:   parsed.ThreadID,
		SessionKey: sessionKey,
	})
	return err
}
