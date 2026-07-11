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
	if err := s.validateIngressDependencies(); err != nil {
		return nil, err
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

	claimed, duplicate, err := s.claimIngress(ctx, normalized)
	if err != nil {
		logger.Error("领取通道消息幂等处理权失败", "err", err)
		return nil, err
	}
	if duplicate != nil {
		logger.Info("忽略重复外部通道消息")
		return duplicate, nil
	}

	if err = s.dispatchIngress(ctx, normalized); err != nil {
		logger.Error("下发通道消息失败", "err", err)
		s.markIngressMessageFailed(ctx, claimed, normalized, err)
		return nil, err
	}
	if err = s.finishAcceptedIngress(ctx, claimed, normalized); err != nil {
		logger.Error("标记通道消息幂等状态失败", "err", err)
		return nil, err
	}

	remembered, err := s.rememberIngressRoutes(ctx, normalized)
	if err != nil {
		logger.Error("记录通道回投目标失败", "err", err)
		return nil, err
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

func (s *IngressService) validateIngressDependencies() error {
	if s.agents == nil {
		return errors.New("ingress service is not configured with agent resolver")
	}
	if s.dm == nil {
		return errors.New("ingress service is not configured with dm handler")
	}
	return nil
}

func (s *IngressService) claimIngress(ctx context.Context, request normalizedIngressRequest) (bool, *IngressResult, error) {
	if s.control == nil || request.reqID == "" {
		return false, nil, nil
	}
	claimed, duplicate, err := s.control.claimIngressMessage(ctx, ingressMessageClaimInput{
		OwnerUserID: request.ownerUserID,
		Channel:     request.channelStored,
		AccountID:   request.accountID,
		ReqID:       request.reqID,
		AgentID:     request.agentID,
		SessionKey:  request.sessionKey,
		RoundID:     request.roundID,
	})
	if err != nil || claimed {
		return claimed, nil, err
	}
	return false, duplicate, nil
}

func (s *IngressService) dispatchIngress(ctx context.Context, request normalizedIngressRequest) error {
	ownerCtx := contextWithIngressOwner(ctx, request.ownerUserID)
	agentValue, err := s.agents.GetAgent(ownerCtx, request.agentID)
	if err != nil {
		return err
	}
	return s.dm.HandleChat(ownerCtx, dmsvc.Request{
		SessionKey:           request.sessionKey,
		AgentID:              request.agentID,
		Content:              request.content,
		RoundID:              request.roundID,
		PermissionMode:       request.permissionMode,
		BroadcastUserMessage: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Metadata: channelmessage.RuntimeMetadata(request.message),
		},
		PermissionHandler:   s.buildPermissionHandler(agentValue, request),
		ExternalReplyTarget: dmExternalReplyTarget(request.rememberedTarget),
	})
}

func (s *IngressService) finishAcceptedIngress(ctx context.Context, claimed bool, request normalizedIngressRequest) error {
	if !claimed {
		return nil
	}
	return s.control.finishIngressMessage(ctx, ingressMessageFinishInput{
		OwnerUserID: request.ownerUserID,
		Channel:     request.channelStored,
		AccountID:   request.accountID,
		ReqID:       request.reqID,
		Status:      ingressMessageStatusAccepted,
	})
}

func (s *IngressService) rememberIngressRoutes(ctx context.Context, request normalizedIngressRequest) (*DeliveryTarget, error) {
	if request.rememberedTarget == nil || s.router == nil {
		return nil, nil
	}
	remembered, err := s.router.RememberRoute(ctx, request.agentID, *request.rememberedTarget)
	if err != nil {
		return nil, err
	}
	_, err = s.router.RememberSessionRoute(ctx, request.agentID, request.sessionKey, *request.rememberedTarget)
	return remembered, err
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
