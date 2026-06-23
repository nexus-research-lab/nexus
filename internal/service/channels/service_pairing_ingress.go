package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *ControlService) ResolveIngressAgent(ctx context.Context, request IngressRequest) (string, error) {
	channelType := normalizeIMChannelType(request.Channel)
	if channelType == "" || channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
		return strings.TrimSpace(request.AgentID), nil
	}
	if _, ok := channelCatalogByType(channelType); !ok {
		return strings.TrimSpace(request.AgentID), nil
	}

	ownerUserID := normalizeChannelOwnerUserID(firstNonEmpty(request.OwnerUserID, authctx.OwnerUserID(ctx)))
	chatType := protocol.NormalizeSessionChatType(request.ChatType)
	accountID := strings.TrimSpace(request.AccountID)
	externalRef := strings.TrimSpace(request.Ref)
	if externalRef == "" {
		return strings.TrimSpace(request.AgentID), nil
	}
	threadID := strings.TrimSpace(request.ThreadID)

	active, err := s.findIngressPairingByTarget(ctx, ownerUserID, channelType, accountID, chatType, externalRef, threadID, PairingStatusActive)
	if err != nil {
		return "", err
	}
	if active != nil {
		if err = s.touchPairing(ctx, ownerUserID, active.PairingID); err != nil {
			return "", err
		}
		return active.AgentID, nil
	}

	candidateAgentID := strings.TrimSpace(request.AgentID)
	if candidateAgentID == "" {
		candidateAgentID, _ = s.defaultAgentForChannel(ctx, ownerUserID, channelType)
	}
	if candidateAgentID == "" && s.agents != nil {
		if defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx); defaultErr == nil && defaultAgent != nil {
			candidateAgentID = defaultAgent.AgentID
		}
	}
	if candidateAgentID == "" {
		return "", errors.New("channel ingress requires an active pairing or agent_id")
	}

	pending := CreatePairingRequest{
		ChannelType:  channelType,
		AccountID:    accountID,
		ChatType:     chatType,
		ExternalRef:  externalRef,
		ThreadID:     ingressPairingThreadID(chatType, threadID),
		ExternalName: strings.TrimSpace(request.ExternalName),
		AgentID:      candidateAgentID,
		Status:       PairingStatusPending,
		Source:       PairingSourceIngress,
	}
	row, err := s.buildPairingRow(ctx, ownerUserID, pending)
	if err != nil {
		return "", err
	}
	created, err := s.upsertPairingRowAndReload(ctx, row)
	if err != nil {
		return "", err
	}
	return "", &pairingApprovalError{
		PairingID: created.PairingID,
		Message:   "IM 对象尚未配对授权，请先在配对控制台批准",
	}
}

func (s *ControlService) findIngressPairingByTarget(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
	chatType string,
	externalRef string,
	threadID string,
	status string,
) (*pairingRow, error) {
	item, err := s.findPairingByTarget(ctx, ownerUserID, channelType, accountID, chatType, externalRef, threadID, status)
	if err != nil || item != nil {
		return item, err
	}
	if usesGroupScopedPairing(chatType, threadID) {
		item, err = s.findPairingByTarget(ctx, ownerUserID, channelType, accountID, chatType, externalRef, "", status)
		if err != nil || item != nil {
			return item, err
		}
	}
	if !usesAccountlessPairingFallback(channelType, accountID) {
		return nil, nil
	}

	// 旧版本配对没有 account_id；单账号型群聊通道允许用空 account_id 兜底。
	item, err = s.findPairingByTarget(ctx, ownerUserID, channelType, "", chatType, externalRef, threadID, status)
	if err != nil || item != nil || !usesGroupScopedPairing(chatType, threadID) {
		return item, err
	}
	return s.findPairingByTarget(ctx, ownerUserID, channelType, "", chatType, externalRef, "", status)
}

func ingressPairingThreadID(chatType string, threadID string) string {
	if usesGroupScopedPairing(chatType, threadID) {
		return ""
	}
	return strings.TrimSpace(threadID)
}

func usesGroupScopedPairing(chatType string, threadID string) bool {
	return protocol.NormalizeSessionChatType(chatType) == "group" && strings.TrimSpace(threadID) != ""
}

func usesAccountlessPairingFallback(channelType string, accountID string) bool {
	if strings.TrimSpace(accountID) == "" {
		return false
	}
	return normalizeIMChannelType(channelType) != ChannelTypeWeixinPersonal
}

func (s *ControlService) touchPairing(ctx context.Context, ownerUserID string, pairingID string) error {
	query := "UPDATE im_pairings SET last_message_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(pairingID))
	return err
}
