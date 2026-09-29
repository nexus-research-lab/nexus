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
			enabled:  strings.TrimSpace(request.AccountID) != "",
		},
		{
			name:     "chat_type",
			provided: protocol.NormalizeSessionChatType(request.ChatType),
			expected: protocol.NormalizeSessionChatType(parsed.ChatType),
			enabled:  strings.TrimSpace(request.ChatType) != "",
		},
		{
			name:     "ref",
			provided: strings.TrimSpace(request.Ref),
			expected: strings.TrimSpace(parsed.Ref),
			enabled:  strings.TrimSpace(request.Ref) != "",
		},
		{
			name:     "thread_id",
			provided: strings.TrimSpace(request.ThreadID),
			expected: strings.TrimSpace(parsed.ThreadID),
			enabled:  strings.TrimSpace(request.ThreadID) != "",
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

	agentID, pairedSessionKey, err := s.resolveIngressSession(ctx, request)
	if err != nil {
		return "", protocol.SessionKey{}, "", err
	}
	if strings.TrimSpace(pairedSessionKey) != "" {
		parsed := protocol.ParseSessionKey(pairedSessionKey)
		return pairedSessionKey, parsed, agentID, nil
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
	agentID, _, err := s.resolveIngressSession(ctx, request)
	return agentID, err
}

func (s *IngressService) resolveIngressSession(ctx context.Context, request IngressRequest) (string, string, error) {
	if s.control != nil {
		agentID, sessionKey, err := s.control.ResolveIngressSession(ctx, request)
		if err != nil || strings.TrimSpace(agentID) != "" {
			return strings.TrimSpace(agentID), strings.TrimSpace(sessionKey), err
		}
	}
	if agentID := strings.TrimSpace(request.AgentID); agentID != "" {
		return agentID, "", nil
	}
	if s.agents == nil {
		return "", "", errors.New("channel ingress 缺少默认 agent 解析器")
	}
	defaultAgent, err := s.agents.GetDefaultAgent(ctx)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(defaultAgent.AgentID), "", nil
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
		if isExternalIngressChannel(channelStored) {
			if target.SessionKey != "" && target.SessionKey != parsed.Raw {
				return nil, errors.New("delivery target session_key 与入站 pairing 不一致")
			}
			// The reply target is a capability of the exact ingress Session. Do
			// not let an adapter omit it or substitute another Session while the
			// visible recipient happens to look the same.
			target.SessionKey = parsed.Raw
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
		target := deliveryTargetFromSessionRef(channelStored, parsed)
		target.SessionKey = parsed.Raw
		return target, nil
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

func isExternalIngressChannel(channel string) bool {
	switch normalizeIMChannelType(channel) {
	case ChannelTypeDiscord, ChannelTypeTelegram, ChannelTypeDingTalk, ChannelTypeWeChat, ChannelTypeWeixinPersonal, ChannelTypeFeishu:
		return true
	default:
		return false
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
