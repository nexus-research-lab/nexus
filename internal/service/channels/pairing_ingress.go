package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type ingressPairingTarget struct {
	ownerUserID string
	channelType string
	accountID   string
	chatType    string
	externalRef string
	threadID    string
}

func (s *ControlService) ResolveIngressAgent(ctx context.Context, request IngressRequest) (string, error) {
	target, pairingRequired := ingressPairingTargetFromRequest(ctx, request)
	if !pairingRequired {
		return strings.TrimSpace(request.AgentID), nil
	}
	active, err := s.findIngressPairingByTarget(
		ctx,
		target.ownerUserID,
		target.channelType,
		target.accountID,
		target.chatType,
		target.externalRef,
		target.threadID,
		PairingStatusActive,
	)
	if err != nil {
		return "", err
	}
	if active != nil {
		if err = s.touchPairing(ctx, target.ownerUserID, active.PairingID); err != nil {
			return "", err
		}
		return active.AgentID, nil
	}
	candidateAgentID := s.ingressPairingCandidateAgent(ctx, request.AgentID, target)
	if candidateAgentID == "" {
		return "", errors.New("channel ingress requires an active pairing or agent_id")
	}
	return s.createPendingIngressPairing(ctx, request, target, candidateAgentID)
}

func ingressPairingTargetFromRequest(ctx context.Context, request IngressRequest) (ingressPairingTarget, bool) {
	channelType := normalizeIMChannelType(request.Channel)
	if channelType == "" || channelType == ChannelTypeInternal || channelType == ChannelTypeWebSocket {
		return ingressPairingTarget{}, false
	}
	if _, ok := channelCatalogByType(channelType); !ok {
		return ingressPairingTarget{}, false
	}
	target := ingressPairingTarget{
		ownerUserID: normalizeChannelOwnerUserID(firstNonEmpty(request.OwnerUserID, authctx.OwnerUserID(ctx))),
		channelType: channelType,
		accountID:   strings.TrimSpace(request.AccountID),
		chatType:    protocol.NormalizeSessionChatType(request.ChatType),
		externalRef: strings.TrimSpace(request.Ref),
		threadID:    strings.TrimSpace(request.ThreadID),
	}
	return target, target.externalRef != ""
}

func (s *ControlService) ingressPairingCandidateAgent(
	ctx context.Context,
	explicitAgentID string,
	target ingressPairingTarget,
) string {
	if agentID := strings.TrimSpace(explicitAgentID); agentID != "" {
		return agentID
	}
	if agentID, _ := s.defaultAgentForChannel(ctx, target.ownerUserID, target.channelType); agentID != "" {
		return agentID
	}
	if s.agents == nil {
		return ""
	}
	defaultAgent, err := s.agents.GetDefaultAgent(ctx)
	if err != nil || defaultAgent == nil {
		return ""
	}
	return strings.TrimSpace(defaultAgent.AgentID)
}

func (s *ControlService) createPendingIngressPairing(
	ctx context.Context,
	request IngressRequest,
	target ingressPairingTarget,
	agentID string,
) (string, error) {
	pending := CreatePairingRequest{
		ChannelType:  target.channelType,
		AccountID:    target.accountID,
		ChatType:     target.chatType,
		ExternalRef:  target.externalRef,
		ThreadID:     ingressPairingThreadID(target.chatType, target.threadID),
		ExternalName: strings.TrimSpace(request.ExternalName),
		AgentID:      agentID,
		Status:       PairingStatusPending,
		Source:       PairingSourceIngress,
	}
	row, err := s.buildPairingRow(ctx, target.ownerUserID, pending)
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
