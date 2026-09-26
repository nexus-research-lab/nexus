// INPUT: owner、Agent、结构化外部 Session 与主动消息正文。
// OUTPUT: active-paired 真实私聊目录、主动投递结果或 fail-closed 的撤销/越权错误。
// POS: IM pairing 到 ingress、Agent 通讯和 Automation 的共用实时授权边界。
package channels

import (
	"context"
	"database/sql"
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

// fallbackPairingSessionMatches permits only legacy, generation-less keys when
// the exact concrete pairing-session row is unavailable. A rotated key must
// never regain authority merely because its platform target still matches.
func fallbackPairingSessionMatches(pairing *pairingRow, parsed protocol.SessionKey, sessionKey string) bool {
	if pairing == nil {
		return false
	}
	if pairingSessionKey(*pairing) == strings.TrimSpace(sessionKey) {
		return true
	}
	// Wildcard pairings use a separate concrete mapping per account/thread.
	// Before that mapping existed, only the original generation-less key may
	// use the target fallback; generated keys require an exact mapping.
	if parsed.Generation != "" {
		return false
	}
	// Once the pairing itself has rotated to a generated key, its old
	// generation-less target-shaped key is no longer a legacy alias. Requiring
	// the current pairing key to be generation-less prevents a stale key from
	// regaining authority through the target fallback after Session deletion or
	// Agent rebinding.
	if current := protocol.ParseSessionKey(pairingSessionKey(*pairing)); current.Generation != "" {
		return false
	}
	if normalizeIMChannelType(pairing.ChannelType) != normalizeIMChannelType(parsed.Channel) ||
		protocol.NormalizeSessionChatType(pairing.ChatType) != protocol.NormalizeSessionChatType(parsed.ChatType) ||
		strings.TrimSpace(pairing.ExternalRef) != strings.TrimSpace(parsed.Ref) {
		return false
	}
	// A generation-less key may use the legacy target fallback only when the
	// pairing itself is a wildcard for the dimensions that differ. The old
	// implementation used OR here, which let an explicit account/topic pairing
	// authorize a different account or topic whenever either dimension differed.
	accountMatches := strings.TrimSpace(pairing.AccountID) == "" ||
		strings.TrimSpace(pairing.AccountID) == strings.TrimSpace(parsed.AccountID)
	threadMatches := strings.TrimSpace(pairing.ThreadID) ==
		ingressPairingThreadID(parsed.ChatType, parsed.ThreadID)
	return accountMatches && threadMatches
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
		sessionKey := pairingSessionKey(row)
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
	pairing, err := s.findPairingBySessionKey(ctx, normalizeChannelOwnerUserID(ownerUserID), sessionKey, PairingStatusActive)
	if err != nil {
		return err
	}
	if pairing == nil {
		pairing, err = s.findIngressPairingByTarget(
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
	}
	if pairing == nil {
		return unavailableExternalSessionGrant("pairing is not active")
	}
	if !fallbackPairingSessionMatches(pairing, parsed, sessionKey) {
		return unavailableExternalSessionGrant("session key is stale or rotated")
	}
	if strings.TrimSpace(pairing.AgentID) != strings.TrimSpace(agentID) {
		return unavailableExternalSessionGrant("pairing is bound to another Agent")
	}
	return nil
}

func (s *ControlService) findPairingBySessionKey(ctx context.Context, ownerUserID, sessionKey, status string) (*pairingRow, error) {
	query := `
	SELECT pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	       agent_id, status, source, session_key, session_materialized, last_message_at, created_at, updated_at, target_room_id, target_conversation_id, binding_version
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + ` AND session_key = ` + s.bind(2) + ` AND status = ` + s.bind(3) + `
	LIMIT 1`
	item, err := scanPairingScanner(s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(sessionKey), normalizePairingStatus(status, PairingStatusActive)))
	if errors.Is(err, sql.ErrNoRows) {
		return s.findPairingSessionBySessionKey(ctx, ownerUserID, sessionKey)
	}
	return item, err
}
