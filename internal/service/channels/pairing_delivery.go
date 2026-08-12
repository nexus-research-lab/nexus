// INPUT: owner、Agent 与 ingress/Automation 使用的结构化外部 session key。
// OUTPUT: 当前精确 active pairing grant，或 fail-closed 的撤销/越权错误。
// POS: IM pairing 到 ingress 能力提升及 Automation create/delivery/retry 的共用实时授权边界。
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

func unavailableExternalSessionGrant(reason string) error {
	return fmt.Errorf("%w: %s", ErrExternalSessionGrantUnavailable, reason)
}

// ValidateAutomationDeliveryGrant 验证定时任务此刻仍持有目标 IM 会话的配对授权。
func (s *ControlService) ValidateAutomationDeliveryGrant(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	sessionKey string,
) error {
	return s.ValidateExternalSessionGrant(ctx, ownerUserID, agentID, sessionKey)
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
