package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *IngressService) resolveSession(ctx context.Context, request IngressRequest) (string, protocol.SessionKey, string, error) {
	if strings.TrimSpace(request.SessionKey) != "" {
		sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
		if err != nil {
			return "", protocol.SessionKey{}, "", err
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent {
			return "", protocol.SessionKey{}, "", errors.New("channel ingress 仅支持 agent session_key")
		}
		if channel := protocol.NormalizeSessionKeyChannelSegment(request.Channel); channel != "" && channel != protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) {
			return "", protocol.SessionKey{}, "", errors.New("channel 与 session_key 不一致")
		}
		if agentID := strings.TrimSpace(request.AgentID); agentID != "" && agentID != parsed.AgentID {
			return "", protocol.SessionKey{}, "", errors.New("agent_id 与 session_key 不一致")
		}
		if accountID := strings.TrimSpace(request.AccountID); accountID != "" && parsed.AccountID != "" && accountID != parsed.AccountID {
			return "", protocol.SessionKey{}, "", errors.New("account_id 与 session_key 不一致")
		}
		return sessionKey, parsed, parsed.AgentID, nil
	}

	channel := protocol.NormalizeSessionKeyChannelSegment(request.Channel)
	if channel == "" {
		return "", protocol.SessionKey{}, "", ErrIngressChannelRequired
	}
	ref := strings.TrimSpace(request.Ref)
	if ref == "" {
		return "", protocol.SessionKey{}, "", ErrIngressRefRequired
	}

	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" && s.control != nil {
		resolvedAgentID, pairErr := s.control.ResolveIngressAgent(ctx, request)
		if pairErr != nil {
			return "", protocol.SessionKey{}, "", pairErr
		}
		agentID = strings.TrimSpace(resolvedAgentID)
	}
	if agentID == "" {
		if s.agents == nil {
			return "", protocol.SessionKey{}, "", errors.New("channel ingress 缺少默认 agent 解析器")
		}
		defaultAgent, err := s.agents.GetDefaultAgent(ctx)
		if err != nil {
			return "", protocol.SessionKey{}, "", err
		}
		agentID = defaultAgent.AgentID
	}
	accountID := strings.TrimSpace(request.AccountID)
	sessionKey := protocol.BuildAgentAccountSessionKey(
		agentID,
		channel,
		protocol.NormalizeSessionChatType(request.ChatType),
		accountID,
		ref,
		strings.TrimSpace(request.ThreadID),
	)
	parsed := protocol.ParseSessionKey(sessionKey)
	return sessionKey, parsed, agentID, nil
}

func (s *IngressService) resolveRememberedTarget(
	channelStored string,
	parsed protocol.SessionKey,
	explicit *DeliveryTarget,
) (*DeliveryTarget, error) {
	if explicit != nil {
		target := explicit.Normalized()
		target.Mode = DeliveryModeExplicit
		if target.Channel == "" {
			target.Channel = channelStored
		}
		if target.Channel == ChannelTypeInternal && target.SessionKey == "" {
			target.SessionKey = parsed.Raw
		}
		if target.Channel == ChannelTypeWeixinPersonal && target.AccountID == "" {
			target.AccountID = strings.TrimSpace(parsed.AccountID)
		}
		if err := target.Validate(); err != nil {
			return nil, err
		}
		return &target, nil
	}

	switch channelStored {
	case ChannelTypeInternal:
		target := DeliveryTarget{
			Mode:       DeliveryModeExplicit,
			Channel:    ChannelTypeInternal,
			To:         parsed.Raw,
			SessionKey: parsed.Raw,
		}
		return &target, nil
	case ChannelTypeTelegram, ChannelTypeDingTalk, ChannelTypeWeChat, ChannelTypeWeixinPersonal, ChannelTypeFeishu:
		return deliveryTargetFromSessionRef(channelStored, parsed), nil
	case ChannelTypeDiscord:
		if parsed.ChatType != "group" {
			return nil, nil
		}
		guildID, channelID := splitDiscordRoute(strings.TrimSpace(parsed.Ref))
		if channelID == "" {
			return nil, nil
		}
		target := DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeDiscord,
			To:        channelID,
			AccountID: guildID,
			ThreadID:  strings.TrimSpace(parsed.ThreadID),
		}
		return &target, nil
	default:
		return nil, nil
	}
}

func deliveryTargetFromSessionRef(channel string, parsed protocol.SessionKey) *DeliveryTarget {
	ref := strings.TrimSpace(parsed.Ref)
	if ref == "" {
		return nil
	}
	target := &DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  channel,
		To:       ref,
		ThreadID: strings.TrimSpace(parsed.ThreadID),
	}
	if channel == ChannelTypeWeixinPersonal {
		target.AccountID = strings.TrimSpace(parsed.AccountID)
	}
	return target
}

func splitDiscordRoute(ref string) (string, string) {
	left, right, found := strings.Cut(strings.TrimSpace(ref), ":")
	if !found {
		return "", strings.TrimSpace(left)
	}
	return strings.TrimSpace(left), strings.TrimSpace(right)
}
