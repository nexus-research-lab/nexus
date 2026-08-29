// INPUT: owner、Agent、结构化外部 Session 与主动消息正文。
// OUTPUT: active-paired 真实私聊目录、主动投递结果或 fail-closed 的撤销/越权错误。
// POS: IM pairing 到 ingress、Agent 通讯和 Automation 的共用实时授权边界。
package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrExternalSessionGrantUnavailable 表示外部会话当前无法证明持有精确 active pairing。
var ErrExternalSessionGrantUnavailable = errors.New("external IM pairing grant is unavailable")

// AgentExternalSession 是当前 Agent 可安全寻址的 active-paired 外部私聊。
type AgentExternalSession struct {
	SessionKey string `json:"session_key"`
	Channel    string `json:"channel"`
	Label      string `json:"label,omitempty"`
	AgentID    string `json:"agent_id"`
}

func unavailableExternalSessionGrant(reason string) error {
	return fmt.Errorf("%w: %s", ErrExternalSessionGrantUnavailable, reason)
}

// ListAgentExternalSessions 列出同 owner、同 Agent 的 active-paired 真实私聊。
// 返回结构化 Session，而不是裸 recipient，后续发送仍会再次校验 pairing。
func (s *ControlService) ListAgentExternalSessions(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	channelType string,
) ([]AgentExternalSession, error) {
	channelType = normalizeIMChannelType(channelType)
	if channelType != "" {
		if _, ok := channelCatalogByType(channelType); !ok ||
			channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
			return nil, unavailableExternalSessionGrant("channel is not an external IM transport")
		}
	}
	rows, err := s.listPairingRows(ctx, normalizeChannelOwnerUserID(ownerUserID), PairingQuery{
		ChannelType: channelType,
		Status:      PairingStatusActive,
		AgentID:     strings.TrimSpace(agentID),
	})
	if err != nil {
		return nil, err
	}
	result := make([]AgentExternalSession, 0, len(rows))
	for _, row := range rows {
		if protocol.NormalizeSessionChatType(row.ChatType) != protocol.RoomTypeDM {
			continue
		}
		sessionKey := protocol.BuildAgentAccountSessionKey(
			row.AgentID,
			protocol.NormalizeSessionKeyChannelSegment(row.ChannelType),
			row.ChatType,
			row.AccountID,
			row.ExternalRef,
			row.ThreadID,
		)
		stored, resolveErr := s.resolveDeliverySession(ctx, sessionKey)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if stored == nil || strings.TrimSpace(stored.SessionKey) != sessionKey ||
			strings.TrimSpace(stored.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		label := strings.TrimSpace(nullStringValue(row.ExternalName))
		if label == "" {
			label = strings.TrimSpace(stored.Title)
		}
		result = append(result, AgentExternalSession{
			SessionKey: sessionKey,
			Channel:    row.ChannelType,
			Label:      label,
			AgentID:    row.AgentID,
		})
	}
	return result, nil
}

// SendAgentExternalSessionMessage 向同一 Agent 已配对的真实外部私聊发送消息。
func (s *ControlService) SendAgentExternalSessionMessage(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	sessionKey string,
	text string,
) (DeliveryResult, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	agentID = strings.TrimSpace(agentID)
	sessionKey = strings.TrimSpace(sessionKey)
	if strings.TrimSpace(text) == "" {
		return DeliveryResult{}, errors.New("external session message content is empty")
	}
	if err := s.ValidateExternalSessionGrant(ctx, ownerUserID, agentID, sessionKey); err != nil {
		return DeliveryResult{}, err
	}
	stored, err := s.resolveDeliverySession(ctx, sessionKey)
	if err != nil {
		return DeliveryResult{}, err
	}
	if stored == nil || strings.TrimSpace(stored.SessionKey) != sessionKey ||
		strings.TrimSpace(stored.AgentID) != agentID {
		return DeliveryResult{}, unavailableExternalSessionGrant("session is not available")
	}
	return s.router.deliverAgentSessionMessage(ctx, agentID, text, sessionKey)
}

func (s *ControlService) resolveDeliverySession(
	ctx context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	if s == nil || s.router == nil {
		return nil, errors.New("external session delivery is not configured")
	}
	return s.router.resolveDeliverySession(ctx, sessionKey)
}

// ValidateExternalSessionGrant 验证结构化外部会话仍精确绑定到当前 owner 与 Agent。
// Automation 投递、IM ingress 能力提升和斜杠审批共用这一条实时撤销边界。
func (s *ControlService) ValidateExternalSessionGrant(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	sessionKey string,
) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return unavailableExternalSessionGrant("authorization requires a structured Agent session_key")
	}
	if strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(agentID) {
		return unavailableExternalSessionGrant("session is bound to another Agent")
	}
	channelType := normalizeIMChannelType(parsed.Channel)
	if channelType == "" || channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
		return unavailableExternalSessionGrant("validation requires an external IM session")
	}
	pairing, err := s.findIngressPairingByTarget(
		ctx,
		normalizeChannelOwnerUserID(ownerUserID),
		channelType,
		strings.TrimSpace(parsed.AccountID),
		protocol.NormalizeSessionChatType(parsed.ChatType),
		strings.TrimSpace(parsed.Ref),
		ingressPairingThreadID(parsed.ChatType, parsed.ThreadID),
		PairingStatusActive,
	)
	if err != nil {
		return err
	}
	if pairing == nil {
		return unavailableExternalSessionGrant("pairing is not active")
	}
	if strings.TrimSpace(pairing.AgentID) != strings.TrimSpace(agentID) {
		return unavailableExternalSessionGrant("pairing is bound to another Agent")
	}
	return nil
}
