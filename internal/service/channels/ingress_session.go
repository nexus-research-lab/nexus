package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *IngressService) resolveSession(ctx context.Context, request IngressRequest) (string, protocol.SessionKey, string, error) {
	if strings.TrimSpace(request.SessionKey) != "" {
		return resolveProvidedIngressSession(request)
	}
	return s.buildIngressSession(ctx, request)
}

func resolveProvidedIngressSession(request IngressRequest) (string, protocol.SessionKey, string, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", protocol.SessionKey{}, "", err
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return "", protocol.SessionKey{}, "", errors.New("channel ingress 仅支持 agent session_key")
	}
	if err = validateIngressSessionIdentity(request, parsed); err != nil {
		return "", protocol.SessionKey{}, "", err
	}
	return sessionKey, parsed, parsed.AgentID, nil
}

func validateIngressSessionIdentity(request IngressRequest, parsed protocol.SessionKey) error {
	constraints := []struct {
		name     string
		provided string
		expected string
		enabled  bool
	}{
		{
			name:     "channel",
			provided: protocol.NormalizeSessionKeyChannelSegment(request.Channel),
			expected: protocol.NormalizeSessionKeyChannelSegment(parsed.Channel),
			enabled:  strings.TrimSpace(request.Channel) != "",
		},
		{
			name:     "agent_id",
			provided: strings.TrimSpace(request.AgentID),
			expected: parsed.AgentID,
			enabled:  strings.TrimSpace(request.AgentID) != "",
		},
		{
			name:     "account_id",
			provided: strings.TrimSpace(request.AccountID),
			expected: parsed.AccountID,
			enabled:  strings.TrimSpace(request.AccountID) != "" && parsed.AccountID != "",
		},
	}
	for _, constraint := range constraints {
		if constraint.enabled && constraint.provided != constraint.expected {
			return fmt.Errorf("%s 与 session_key 不一致", constraint.name)
		}
	}
	return nil
}

func (s *IngressService) buildIngressSession(ctx context.Context, request IngressRequest) (string, protocol.SessionKey, string, error) {
	channel := protocol.NormalizeSessionKeyChannelSegment(request.Channel)
	if channel == "" {
		return "", protocol.SessionKey{}, "", ErrIngressChannelRequired
	}
	ref := strings.TrimSpace(request.Ref)
	if ref == "" {
		return "", protocol.SessionKey{}, "", ErrIngressRefRequired
	}

	agentID, err := s.resolveIngressAgent(ctx, request)
	if err != nil {
		return "", protocol.SessionKey{}, "", err
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

func (s *IngressService) resolveIngressAgent(ctx context.Context, request IngressRequest) (string, error) {
	if agentID := strings.TrimSpace(request.AgentID); agentID != "" {
		return agentID, nil
	}
	if s.control != nil {
		agentID, err := s.control.ResolveIngressAgent(ctx, request)
		if err != nil || strings.TrimSpace(agentID) != "" {
			return strings.TrimSpace(agentID), err
		}
	}
	if s.agents == nil {
		return "", errors.New("channel ingress 缺少默认 agent 解析器")
	}
	defaultAgent, err := s.agents.GetDefaultAgent(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(defaultAgent.AgentID), nil
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
