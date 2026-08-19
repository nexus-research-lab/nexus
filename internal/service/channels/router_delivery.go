package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// DeliverMessage 按目标模式解析并完成消息投递，返回平台回执。
func (r *Router) DeliverMessage(ctx context.Context, agentID string, text string, target DeliveryTarget) (DeliveryResult, error) {
	normalized := target.Normalized()
	routeSessionKey := normalized.SessionKey
	if strings.TrimSpace(text) == "" || normalized.Mode == DeliveryModeNone {
		return DeliveryResult{Target: normalized}, nil
	}
	normalized, err := r.resolveDeliveryTarget(ctx, agentID, normalized)
	if err != nil {
		return DeliveryResult{}, err
	}
	if normalized.SessionKey == "" {
		normalized.SessionKey = routeSessionKey
	}
	if err := normalized.Validate(); err != nil {
		return DeliveryResult{}, err
	}
	result, err := r.sendDelivery(ctx, agentID, text, normalized)
	if err != nil {
		// 适配器可在失败时返回已经修正的权威目标。例如个人微信确认
		// context_token 失效后会清空它，避免 Automation 重试继续复用坏上下文。
		if strings.TrimSpace(result.Target.Mode) != "" {
			result = normalizeDeliveryResult(result, normalized)
			if validateErr := result.Target.Validate(); validateErr == nil {
				if rememberErr := r.rememberDeliveryTarget(ctx, agentID, routeSessionKey, result.Target); rememberErr != nil {
					return result, errors.Join(err, rememberErr)
				}
			}
		}
		return result, err
	}
	result = normalizeDeliveryResult(result, normalized)
	if err = r.rememberDeliveryTarget(ctx, agentID, routeSessionKey, result.Target); err != nil {
		return DeliveryResult{}, err
	}
	r.logDeliverySuccess(ctx, agentID, text, result)
	return result, nil
}

// DeliverAutomationResult 把一次任务结果同时写入其逻辑 Nexus 会话并发送到目标通道。
// 外部平台失败时保留已经完成的会话投影，后续重试依靠稳定 run_id 更新同一条消息。
func (r *Router) DeliverAutomationResult(
	ctx context.Context,
	producerAgentID string,
	text string,
	target DeliveryTarget,
	delivery AutomationDeliveryContext,
) (DeliveryResult, error) {
	delivery = delivery.Normalized()
	if err := delivery.Validate(); err != nil {
		return DeliveryResult{}, err
	}
	normalized := target.Normalized()
	if strings.TrimSpace(delivery.ProducerAgentID) == "" {
		delivery.ProducerAgentID = strings.TrimSpace(producerAgentID)
	}
	routeAgentID := automationDeliveryRouteAgentID(normalized, producerAgentID)
	routeSessionKey := normalized.SessionKey
	if strings.TrimSpace(text) == "" || normalized.Mode == DeliveryModeNone {
		return DeliveryResult{Target: normalized}, nil
	}
	resolved, err := r.resolveDeliveryTarget(ctx, routeAgentID, normalized)
	if err != nil {
		return DeliveryResult{}, err
	}
	if strings.TrimSpace(resolved.SessionKey) == "" {
		resolved.SessionKey = routeSessionKey
	}
	if err = resolved.Validate(); err != nil {
		return DeliveryResult{}, err
	}

	unlock := r.lockAutomationProjection(routeAgentID, delivery.RunID)
	defer unlock()
	projection, projectErr := r.projectAutomationResult(ctx, producerAgentID, text, resolved, delivery)
	if projectErr != nil {
		return DeliveryResult{Target: resolved}, projectErr
	}
	if isSessionDeliveryChannel(resolved.Channel) {
		result := normalizeDeliveryResult(projection, resolved)
		if err = r.rememberDeliveryTarget(ctx, routeAgentID, routeSessionKey, result.Target); err != nil {
			return result, err
		}
		r.logDeliverySuccess(ctx, routeAgentID, text, result)
		return result, nil
	}

	result, sendErr := r.sendDelivery(ctx, routeAgentID, text, resolved)
	result = normalizeDeliveryResult(result, resolved)
	if sendErr != nil {
		if strings.TrimSpace(result.Target.Mode) != "" {
			if validateErr := result.Target.Validate(); validateErr == nil {
				if rememberErr := r.rememberDeliveryTarget(ctx, routeAgentID, routeSessionKey, result.Target); rememberErr != nil {
					return result, errors.Join(sendErr, rememberErr)
				}
			}
		}
		return result, sendErr
	}
	if err = r.rememberDeliveryTarget(ctx, routeAgentID, routeSessionKey, result.Target); err != nil {
		return DeliveryResult{}, err
	}
	if receiptErr := r.attachAutomationExternalReceipt(ctx, routeAgentID, result, delivery); receiptErr != nil {
		r.loggerFor(ctx).Warn("Automation 外部投递回执写入会话失败",
			"agent_id", routeAgentID,
			"job_id", delivery.JobID,
			"run_id", delivery.RunID,
			"session_key", result.Target.SessionKey,
			"err", receiptErr,
		)
	}
	r.logDeliverySuccess(ctx, routeAgentID, text, result)
	return result, nil
}

func automationDeliveryRouteAgentID(target DeliveryTarget, fallback string) string {
	parsed := protocol.ParseSessionKey(target.SessionKey)
	if parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		strings.TrimSpace(parsed.AgentID) != "" {
		return strings.TrimSpace(parsed.AgentID)
	}
	return strings.TrimSpace(fallback)
}

func (r *Router) lockAutomationProjection(agentID string, runID string) func() {
	key := strings.TrimSpace(agentID) + "/" + strings.TrimSpace(runID)
	value, _ := r.projectionLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func isSessionDeliveryChannel(channel string) bool {
	switch normalizeChannelType(channel) {
	case ChannelTypeWebSocket, ChannelTypeInternal:
		return true
	default:
		return false
	}
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
		return DeliveryResult{Target: target}, err
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
		return normalizeDeliveryResult(result, target), err
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
		lastTarget, err := r.GetSessionRoute(ctx, agentID, normalized.SessionKey)
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
