// INPUT: Validated external ingress and current pairing.
// OUTPUT: Existing DM admission with host-persisted human input evidence.
// POS: Channels ingress boundary before runtime dispatch.
package channels

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func dmExternalReplyTarget(target *DeliveryTarget) *dmsvc.ExternalReplyTarget {
	if target == nil {
		return nil
	}
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone {
		return nil
	}
	switch normalized.Channel {
	case "", ChannelTypeWebSocket, ChannelTypeInternal:
		return nil
	}
	return &dmsvc.ExternalReplyTarget{
		PairingID: normalized.PairingID, BindingVersion: normalized.BindingVersion,
		Mode:           normalized.Mode,
		Channel:        normalized.Channel,
		To:             normalized.To,
		AccountID:      normalized.AccountID,
		ThreadID:       normalized.ThreadID,
		SessionKey:     normalized.SessionKey,
		ContextToken:   normalized.ContextToken,
		ReplyContextID: normalized.ReplyContextID,
		StreamID:       normalized.StreamID,
	}
}

// Accept 受理一条外部通道消息。
func (s *IngressService) Accept(ctx context.Context, request IngressRequest) (*IngressResult, error) {
	normalized, err := s.normalizeRequest(ctx, request)
	if err != nil {
		return nil, &channelcontract.RetryableIngressError{Err: err}
	}
	if err := s.validateIngressDependencies(); err != nil {
		return nil, &channelcontract.RetryableIngressError{Err: err}
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
	defer s.recovery.Notify()
	if duplicate != nil && (claimed || errors.Is(err, ErrIngressOutcomeUnknown)) {
		// 重投不能生成新轮次或把未确认输入迁到另一个 Session。
		normalized.roundID, normalized.sessionKey, normalized.agentID = duplicate.RoundID, duplicate.SessionKey, duplicate.AgentID
	}
	if errors.Is(err, ErrIngressOutcomeUnknown) && duplicate != nil {
		if recovered, recoverErr := s.recoverIngress(ctx, normalized); recoverErr == nil && recovered {
			return duplicate, nil
		}
	}
	if err != nil {
		logger.Error("领取通道消息幂等处理权失败", "err", err)
		if duplicate == nil {
			return nil, &channelcontract.RetryableIngressError{Err: err}
		}
		return nil, err
	}
	if duplicate != nil && !claimed {
		logger.Info("忽略重复外部通道消息")
		return duplicate, nil
	}
	remembered, err := s.rememberIngressRoutes(ctx, normalized)
	if err != nil {
		logger.Error("记录通道回投目标失败", "err", err)
		return nil, &channelcontract.RetryableIngressError{Err: err}
	}
	if claimed {
		if err = s.control.beginIngressDispatch(ctx, normalized); err != nil {
			return nil, err
		}
	}
	s.bindRuntimePermissionSession(normalized)

	command, err := s.handleIngressCommand(ctx, normalized)
	if err != nil {
		logger.Error("处理通道控制命令失败", "err", err)
		// 控制命令可能已产生副作用，未知结果不能释放为可重试。
		return nil, err
	}
	if command != nil {
		// 先保存命令已执行的事实；通知失败不能重复审批或执行下一条命令。
		if err = s.finishAcceptedIngress(ctx, claimed, normalized); err != nil {
			logger.Error("标记通道控制命令幂等状态失败", "err", err)
			return nil, err
		}
		if err = s.replyToIngressCommand(ctx, normalized, command.Reply); err != nil {
			logger.Error("回投通道控制命令结果失败", "err", err)
			return nil, err
		}
		s.notifyExternalSessionUpdated(ctx, normalized)
		return acceptedIngressResult(normalized, remembered), nil
	}

	if err = s.dispatchIngress(ctx, normalized); err != nil {
		logger.Error("下发通道消息失败", "err", err)
		// DM 返回失败不证明没有部分受理；重投仅核验原轮次。
		return nil, err
	}
	if s.control != nil && (normalized.pairing == nil || normalized.pairing.TargetRoomID == "") {
		if err = s.control.MarkIngressSessionMaterialized(ctx, normalized.ownerUserID, normalized.sessionKey); err != nil {
			logger.Error("记录通道 Session 物化状态失败", "err", err)
			// DM 已受理，不能因投影失败释放领取权并再次执行。
			return nil, err
		}
	}
	if err = s.finishAcceptedIngress(ctx, claimed, normalized); err != nil {
		logger.Error("标记通道消息幂等状态失败", "err", err)
		return nil, err
	}

	logger.Info("通道消息已进入 DM 主链",
		"remembered_delivery", remembered != nil,
	)
	s.notifyExternalSessionUpdated(ctx, normalized)

	return acceptedIngressResult(normalized, remembered), nil
}

func acceptedIngressResult(request normalizedIngressRequest, remembered *DeliveryTarget) *IngressResult {
	return &IngressResult{
		Channel:            request.channelStored,
		AgentID:            request.agentID,
		SessionKey:         request.sessionKey,
		RoundID:            request.roundID,
		ReqID:              request.reqID,
		RememberedDelivery: remembered,
		Message:            request.message,
	}
}

func (s *IngressService) handleIngressCommand(
	ctx context.Context,
	request normalizedIngressRequest,
) (*IngressCommandResult, error) {
	ingress := IngressCommandRequest{
		OwnerUserID: request.ownerUserID,
		AgentID:     request.agentID,
		SessionKey:  request.permissionSessionKey(),
		Content:     request.content,
	}
	if ambiguity, err := s.permissionCommandAmbiguity(
		contextWithIngressOwner(ctx, request.ownerUserID),
		ingress,
	); err != nil || ambiguity != nil {
		return ambiguity, err
	}
	if result := s.handleRuntimePermissionCommand(ctx, request); result != nil {
		return result, nil
	}
	if s.commands == nil {
		return nil, nil
	}
	result, err := s.commands.HandleIngressCommand(contextWithIngressOwner(ctx, request.ownerUserID), ingress)
	if err != nil || !result.Handled {
		return nil, err
	}
	return &result, nil
}

func (s *IngressService) replyToIngressCommand(ctx context.Context, request normalizedIngressRequest, reply string) error {
	if strings.TrimSpace(reply) == "" || s.router == nil || request.rememberedTarget == nil {
		return nil
	}
	_, err := s.router.DeliverMessage(
		contextWithIngressOwner(ctx, request.ownerUserID),
		request.agentID,
		reply,
		*request.rememberedTarget,
	)
	return err
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
		BindingVersion: request.bindingVersion(),
		Content:        request.content,
		OwnerUserID:    request.ownerUserID,
		Channel:        request.channelStored,
		AccountID:      request.accountID,
		ReqID:          request.reqID,
		AgentID:        request.agentID,
		SessionKey:     request.sessionKey,
		RoundID:        request.roundID,
	})
	return claimed, duplicate, err
}

func (s *IngressService) dispatchIngress(ctx context.Context, request normalizedIngressRequest) error {
	if request.pairing != nil && request.pairing.TargetRoomID != "" {
		return s.dispatchRoomIngress(contextWithIngressOwner(ctx, request.ownerUserID), request)
	}
	if request.trustedExternalInteractive && s.control != nil {
		if err := s.control.recordDeliveryInput(ctx, request); err != nil {
			return err
		}
	}
	ownerCtx := contextWithIngressOwner(ctx, request.ownerUserID)
	agentValue, err := s.agents.GetAgent(ownerCtx, request.agentID)
	if err != nil {
		return err
	}
	return s.dm.HandleChat(ownerCtx, dmsvc.Request{
		SessionKey:                        request.sessionKey,
		AgentID:                           request.agentID,
		Content:                           request.content,
		RoundID:                           request.roundID,
		ExecutionOrigin:                   "channel",
		TrustedExternalInteractiveContext: request.trustedExternalInteractive,
		PermissionMode:                    request.permissionMode,
		BroadcastUserMessage:              true,
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
	// Route persistence precedes DM dispatch so the exact return address is
	// durable even if runtime startup later fails. The first ingress has not yet
	// materialized its workspace Session at this point; defer only the projection
	// read for these two bookkeeping writes. The actual reply goes through the
	// normal send context after DM admission and remains fail-closed.
	routeCtx := context.WithValue(
		contextWithIngressOwner(ctx, request.ownerUserID),
		unmaterializedExternalSessionKey{},
		true,
	)
	remembered, err := s.router.RememberRoute(routeCtx, request.agentID, *request.rememberedTarget)
	if err != nil {
		return nil, err
	}
	_, err = s.router.RememberSessionRoute(routeCtx, request.agentID, request.sessionKey, *request.rememberedTarget)
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

// recoverIngress 为 DM 核验持久输入；Room 复用已冻结命令完成幂等受理，不生成新输入。
func (s *IngressService) recoverIngress(ctx context.Context, request normalizedIngressRequest) (bool, error) {
	if s.rooms != nil {
		ownerCtx := contextWithIngressOwner(ctx, request.ownerUserID)
		route, err := s.roomIngressRoute(ownerCtx, request.roundID, request.agentID)
		if err != nil {
			return false, err
		}
		if route != nil {
			request.rememberedTarget = &route.target
			request.content = route.content
			request.pairing = &pairingRow{PairingID: route.target.PairingID, BindingVersion: route.target.BindingVersion, TargetRoomID: route.roomID, TargetConversationID: route.conversationID}
			if err := s.dispatchRoomIngress(ownerCtx, request); err != nil {
				return false, err
			}
			return true, s.finishAcceptedIngress(ownerCtx, true, request)
		}
	}

	if s.readRoundIndex == nil {
		return false, nil
	}
	ownerCtx := contextWithIngressOwner(ctx, request.ownerUserID)
	index, err := s.readRoundIndex(ownerCtx, request.sessionKey)
	if err != nil || index == nil {
		return false, err
	}
	for _, round := range index.Items {
		if round.RoundID != request.roundID || !round.HasUserMessage {
			continue
		}
		if err := s.control.MarkIngressSessionMaterialized(ownerCtx, request.ownerUserID, request.sessionKey); err != nil {
			return false, err
		}
		return true, s.finishAcceptedIngress(ownerCtx, true, request)
	}
	return false, nil
}
