package channels

import (
	"context"
	"fmt"
	"strings"
)

// DeliverMessage 按目标模式解析并完成消息投递，返回平台回执。
func (r *Router) DeliverMessage(ctx context.Context, agentID string, text string, target DeliveryTarget) (DeliveryResult, error) {
	normalized := target.Normalized()
	if strings.TrimSpace(text) == "" || normalized.Mode == DeliveryModeNone {
		return DeliveryResult{Target: normalized}, nil
	}
	normalized, err := r.resolveDeliveryTarget(ctx, agentID, normalized)
	if err != nil {
		return DeliveryResult{}, err
	}
	if err := normalized.Validate(); err != nil {
		return DeliveryResult{}, err
	}
	result, err := r.sendDelivery(ctx, agentID, text, normalized)
	if err != nil {
		return DeliveryResult{}, err
	}
	if err = r.rememberDeliveryTarget(ctx, agentID, target.SessionKey, normalized); err != nil {
		return DeliveryResult{}, err
	}
	result = normalizeDeliveryResult(result, normalized)
	r.logDeliverySuccess(ctx, agentID, text, result)
	return result, nil
}

func (r *Router) resolveDeliveryTarget(
	ctx context.Context,
	agentID string,
	target DeliveryTarget,
) (DeliveryTarget, error) {
	if target.Mode != DeliveryModeLast {
		return target, nil
	}
	lastTarget, err := r.GetSessionRoute(ctx, agentID, target.SessionKey)
	if err != nil {
		r.loggerFor(ctx).Error("读取最近投递目标失败",
			"agent_id", agentID,
			"session_key", target.SessionKey,
			"err", err,
		)
		return DeliveryTarget{}, err
	}
	if lastTarget == nil && strings.TrimSpace(target.SessionKey) != "" {
		lastTarget, err = r.GetLastRoute(ctx, agentID)
		if err != nil {
			r.loggerFor(ctx).Error("读取 agent 最近投递目标失败", "agent_id", agentID, "err", err)
			return DeliveryTarget{}, err
		}
	}
	if lastTarget != nil {
		return lastTarget.Normalized(), nil
	}
	err = fmt.Errorf("last delivery target is not available for agent: %s", strings.TrimSpace(agentID))
	r.loggerFor(ctx).Warn("最近投递目标不存在",
		"agent_id", agentID,
		"session_key", target.SessionKey,
		"err", err,
	)
	return DeliveryTarget{}, err
}

func (r *Router) sendDelivery(
	ctx context.Context,
	agentID string,
	text string,
	target DeliveryTarget,
) (DeliveryResult, error) {
	channel := r.channelForDelivery(ctx, agentID, target.Channel)
	if channel == nil {
		err := fmt.Errorf("delivery sender is not configured for channel: %s", target.Channel)
		r.loggerFor(ctx).Error("投递通道未配置", "agent_id", agentID, "channel", target.Channel, "err", err)
		return DeliveryResult{}, err
	}
	result, err := sendDeliveryMessage(ctx, channel, agentID, target, text)
	if err != nil {
		r.loggerFor(ctx).Error("文本投递失败",
			"agent_id", agentID,
			"channel", target.Channel,
			"to", target.To,
			"thread_id", target.ThreadID,
			"err", err,
		)
		return DeliveryResult{}, err
	}
	return result, nil
}

func (r *Router) rememberDeliveryTarget(
	ctx context.Context,
	agentID string,
	sessionKey string,
	target DeliveryTarget,
) error {
	if strings.TrimSpace(agentID) == "" {
		return nil
	}
	if strings.TrimSpace(sessionKey) != "" {
		_, err := r.RememberSessionRoute(ctx, agentID, sessionKey, target)
		return err
	}
	_, err := r.RememberRoute(ctx, agentID, target)
	return err
}

func normalizeDeliveryResult(result DeliveryResult, fallback DeliveryTarget) DeliveryResult {
	if strings.TrimSpace(result.Target.Mode) == "" {
		result.Target = fallback
	} else {
		result.Target = result.Target.Normalized()
	}
	return result
}

func (r *Router) logDeliverySuccess(ctx context.Context, agentID string, text string, result DeliveryResult) {
	logArgs := []any{
		"agent_id", agentID,
		"channel", result.Target.Channel,
		"to", result.Target.To,
		"thread_id", result.Target.ThreadID,
		"chars", len([]rune(strings.TrimSpace(text))),
	}
	if result.Receipt != nil {
		logArgs = append(logArgs,
			"primary_platform_message_id", result.Receipt.PrimaryPlatformMessageID,
			"platform_message_ids", result.Receipt.PlatformMessageIDs,
		)
	}
	r.loggerFor(ctx).Info("文本投递成功", logArgs...)
}

func sendDeliveryMessage(
	ctx context.Context,
	channel DeliveryChannel,
	agentID string,
	target DeliveryTarget,
	text string,
) (DeliveryResult, error) {
	if scoped, ok := channel.(agentScopedDeliveryChannel); ok {
		return scoped.SendAgentDeliveryMessage(ctx, agentID, target, text)
	}
	return channel.SendDeliveryMessage(ctx, target, text)
}

// SetTyping 按目标模式发送或取消通道输入状态；不支持 typing 的通道直接忽略。
func (r *Router) SetTyping(ctx context.Context, agentID string, target DeliveryTarget, active bool) error {
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone {
		return nil
	}
	if normalized.Mode == DeliveryModeLast {
		lastTarget, err := r.GetLastRoute(ctx, agentID)
		if err != nil {
			r.loggerFor(ctx).Warn("读取 typing 最近投递目标失败",
				"agent_id", agentID,
				"err", err,
			)
			return err
		}
		if lastTarget == nil {
			return nil
		}
		normalized = lastTarget.Normalized()
	}
	if err := normalized.Validate(); err != nil {
		return err
	}
	channel := r.channelForDelivery(ctx, agentID, normalized.Channel)
	if channel == nil {
		return nil
	}
	typingChannel, ok := channel.(typingDeliveryChannel)
	if !ok {
		return nil
	}
	if err := typingChannel.SendDeliveryTyping(ctx, normalized, active); err != nil {
		r.loggerFor(ctx).Warn("通道 typing 状态投递失败",
			"agent_id", agentID,
			"channel", normalized.Channel,
			"to", normalized.To,
			"active", active,
			"err", err,
		)
		return err
	}
	r.loggerFor(ctx).Debug("通道 typing 状态已投递",
		"agent_id", agentID,
		"channel", normalized.Channel,
		"to", normalized.To,
		"active", active,
	)
	return nil
}
