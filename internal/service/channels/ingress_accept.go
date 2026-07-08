package channels

import (
	"context"
	"errors"
	"unicode/utf8"

	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// Accept 受理一条外部通道消息。
func (s *IngressService) Accept(ctx context.Context, request IngressRequest) (*IngressResult, error) {
	normalized, err := s.normalizeRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.agents == nil {
		return nil, errors.New("ingress service is not configured with agent resolver")
	}
	if s.dm == nil {
		return nil, errors.New("ingress service is not configured with dm handler")
	}

	logger := s.loggerFor(ctx).With(
		"channel", normalized.channelStored,
		"account_id", normalized.accountID,
		"agent_id", normalized.agentID,
		"session_key", normalized.sessionKey,
		"round_id", normalized.roundID,
		"req_id", normalized.reqID,
	)
	logger.Info("受理外部通道消息",
		"content_chars", utf8.RuneCountInString(normalized.content),
		"platform_message_id", normalized.messageID(),
	)

	claimedIngress := false
	if s.control != nil && normalized.reqID != "" {
		claimed, duplicate, claimErr := s.control.claimIngressMessage(ctx, ingressMessageClaimInput{
			OwnerUserID: normalized.ownerUserID,
			Channel:     normalized.channelStored,
			AccountID:   normalized.accountID,
			ReqID:       normalized.reqID,
			AgentID:     normalized.agentID,
			SessionKey:  normalized.sessionKey,
			RoundID:     normalized.roundID,
		})
		if claimErr != nil {
			logger.Error("领取通道消息幂等处理权失败", "err", claimErr)
			return nil, claimErr
		}
		if !claimed {
			logger.Info("忽略重复外部通道消息")
			return duplicate, nil
		}
		claimedIngress = true
	}

	ownerCtx := contextWithIngressOwner(ctx, normalized.ownerUserID)
	agentValue, err := s.agents.GetAgent(ownerCtx, normalized.agentID)
	if err != nil {
		logger.Error("解析通道消息目标 Agent 失败", "err", err)
		s.markIngressMessageFailed(ctx, claimedIngress, normalized, err)
		return nil, err
	}
	if err = s.dm.HandleChat(ownerCtx, dmsvc.Request{
		SessionKey:           normalized.sessionKey,
		AgentID:              normalized.agentID,
		Content:              normalized.content,
		RoundID:              normalized.roundID,
		PermissionMode:       normalized.permissionMode,
		BroadcastUserMessage: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Metadata: channelmessage.RuntimeMetadata(normalized.message),
		},
		PermissionHandler:   s.buildPermissionHandler(agentValue, normalized),
		ExternalReplyTarget: dmExternalReplyTarget(normalized.rememberedTarget),
	}); err != nil {
		logger.Error("下发通道消息失败", "err", err)
		s.markIngressMessageFailed(ctx, claimedIngress, normalized, err)
		return nil, err
	}
	if claimedIngress {
		if err = s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
			OwnerUserID: normalized.ownerUserID,
			Channel:     normalized.channelStored,
			AccountID:   normalized.accountID,
			ReqID:       normalized.reqID,
			Status:      ingressMessageStatusAccepted,
		}); err != nil {
			logger.Error("标记通道消息幂等状态失败", "err", err)
			return nil, err
		}
	}

	var remembered *DeliveryTarget
	if normalized.rememberedTarget != nil && s.router != nil {
		remembered, err = s.router.RememberRoute(ctx, normalized.agentID, *normalized.rememberedTarget)
		if err != nil {
			logger.Error("记录通道回投目标失败", "err", err)
			return nil, err
		}
		if _, err = s.router.RememberSessionRoute(ctx, normalized.agentID, normalized.sessionKey, *normalized.rememberedTarget); err != nil {
			logger.Error("记录通道 session 回投目标失败", "err", err)
			return nil, err
		}
	}
	logger.Info("通道消息已进入 DM 主链",
		"remembered_delivery", remembered != nil,
	)
	s.notifyExternalSessionUpdated(ctx, normalized)

	return &IngressResult{
		Channel:            normalized.channelStored,
		AgentID:            normalized.agentID,
		SessionKey:         normalized.sessionKey,
		RoundID:            normalized.roundID,
		ReqID:              normalized.reqID,
		RememberedDelivery: remembered,
		Message:            normalized.message,
	}, nil
}

func (s *IngressService) notifyExternalSessionUpdated(ctx context.Context, request normalizedIngressRequest) {
	if s.notifier == nil || !shouldNotifyExternalSessionUpdate(request.channelStored) {
		return
	}
	s.notifier.NotifyExternalSessionUpdated(ctx, request.agentID, request.sessionKey)
}

func shouldNotifyExternalSessionUpdate(channel string) bool {
	normalized := normalizeChannelType(channel)
	return normalized != "" && normalized != ChannelTypeInternal && normalized != ChannelTypeWebSocket
}

func (s *IngressService) markIngressMessageFailed(ctx context.Context, claimed bool, request normalizedIngressRequest, err error) {
	if !claimed || s.control == nil || err == nil {
		return
	}
	message := err.Error()
	if finishErr := s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
		OwnerUserID:  request.ownerUserID,
		Channel:      request.channelStored,
		AccountID:    request.accountID,
		ReqID:        request.reqID,
		Status:       ingressMessageStatusFailed,
		ErrorMessage: &message,
	}); finishErr != nil {
		s.loggerFor(ctx).Warn("标记通道消息失败幂等状态失败",
			"channel", request.channelStored,
			"req_id", request.reqID,
			"err", finishErr,
		)
	}
}
