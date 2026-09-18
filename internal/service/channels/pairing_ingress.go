// INPUT: 规范化后的外部消息入口及其 owner/channel/chat 标识。
// OUTPUT: 已授权 Agent，或可审批的 pending pairing 与消息时间更新。
// POS: Pairing ingress writer，与人工 pairing 变更共享 owner 级写锁。
package channels

import (
	"context"
	"database/sql"
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
	agentID, _, err := s.ResolveIngressSession(ctx, request)
	return agentID, err
}

// ResolveIngressSession resolves both the current Agent and the persisted
// Session identity for an active pairing. A deleted materialized Session is
// replaced with a new generation; the old key remains fenced by Session
// deletion tombstones.
func (s *ControlService) ResolveIngressSession(ctx context.Context, request IngressRequest) (string, string, error) {
	target, pairingRequired := ingressPairingTargetFromRequest(ctx, request)
	if !pairingRequired {
		return strings.TrimSpace(request.AgentID), "", nil
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
		return "", "", err
	}
	if active != nil {
		if err = s.touchPairing(ctx, target.ownerUserID, active.PairingID); err != nil {
			return "", "", err
		}
		// Group pairings may intentionally be account/topic wildcards. Their
		// persisted key belongs to the wildcard row, while each concrete ingress
		// still needs its own platform-scoped Session identity.
		if active.AccountID != target.accountID || active.ThreadID != strings.TrimSpace(target.threadID) {
			concreteTarget := pairingSessionTargetFromIngress(target.ownerUserID, target)
			sessionKey, resolveErr := s.resolveConcretePairingSession(ctx, concreteTarget, *active)
			return active.AgentID, sessionKey, resolveErr
		}
		sessionKey := pairingSessionKey(*active)
		if s.router != nil && s.router.sessionProjectionConfigured() {
			var stored *protocol.Session
			if active.SessionMaterialized {
				var resolveErr error
				stored, resolveErr = s.resolveDeliverySession(ctx, sessionKey)
				if resolveErr != nil {
					return "", "", resolveErr
				}
			}
			deleted, deletionErr := s.router.sessionDeleted(ctx, sessionKey)
			if deletionErr != nil {
				return "", "", deletionErr
			}
			if deleted || (active.SessionMaterialized && stored == nil) {
				generation := s.idFactory("session")
				sessionKey = protocol.BuildAgentAccountSessionKeyWithGeneration(
					active.AgentID,
					protocol.NormalizeSessionKeyChannelSegment(active.ChannelType),
					active.ChatType,
					active.AccountID,
					active.ExternalRef,
					active.ThreadID,
					generation,
				)
				if sessionKey, err = s.rotatePairingSessionKey(ctx, target.ownerUserID, active.PairingID, pairingSessionKey(*active), sessionKey); err != nil {
					return "", "", err
				}
			}
		}
		return active.AgentID, sessionKey, nil
	}
	candidateAgentID := s.ingressPairingCandidateAgent(ctx, request.AgentID, target)
	if candidateAgentID == "" {
		return "", "", errors.New("channel ingress requires an active pairing or agent_id")
	}
	agentID, err := s.createPendingIngressPairing(ctx, request, target, candidateAgentID)
	return agentID, "", err
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
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	unlock := s.lockPairingMutation(ownerUserID)
	defer unlock()

	query := "UPDATE im_pairings SET last_message_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, ownerUserID, strings.TrimSpace(pairingID))
	return err
}

func (s *ControlService) rotatePairingSessionKey(ctx context.Context, ownerUserID, pairingID, expectedSessionKey, sessionKey string) (string, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	pairingID = strings.TrimSpace(pairingID)
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return "", errors.New("pairing session_key is required")
	}
	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockPairing := s.lockPairingMutation(ownerUserID)
	defer unlockPairing()
	currentSessionKey := sessionKey
	_, err := s.withChannelControlMutation(ctx, ownerUserID, 0, func(tx *sql.Tx) error {
		if _, updateErr := tx.ExecContext(ctx, "UPDATE im_deliveries SET return_revoked=1 WHERE owner_user_id="+s.bind(1)+" AND pairing_id="+s.bind(2), ownerUserID, pairingID); updateErr != nil {
			return updateErr
		}
		result, updateErr := tx.ExecContext(ctx, "UPDATE im_pairings SET session_key="+s.bind(1)+", session_materialized="+s.bind(2)+", updated_at=CURRENT_TIMESTAMP WHERE owner_user_id="+s.bind(3)+" AND pairing_id="+s.bind(4)+" AND status="+s.bind(5)+" AND session_key="+s.bind(6), sessionKey, false, ownerUserID, pairingID, PairingStatusActive, strings.TrimSpace(expectedSessionKey))
		if updateErr != nil {
			return updateErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if affected == 0 {
			current, loadErr := s.getPairingRowFrom(ctx, tx, ownerUserID, pairingID)
			if loadErr != nil {
				return loadErr
			}
			if current == nil || current.Status != PairingStatusActive {
				return ErrPairingNotFound
			}
			currentSessionKey = pairingSessionKey(*current)
		}
		return nil
	})
	return currentSessionKey, err
}

// MarkIngressSessionMaterialized records that DM admission has successfully
// created or updated the current pairing Session. It distinguishes an initial
// pairing (which has no Session yet) from a deleted Session that needs rotation.
func (s *ControlService) MarkIngressSessionMaterialized(ctx context.Context, ownerUserID, sessionKey string) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	return s.markIngressSessionMaterialized(ctx, ownerUserID, sessionKey)
}

func (s *ControlService) markIngressSessionMaterialized(ctx context.Context, ownerUserID, sessionKey string) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE im_pairings SET session_materialized=1, updated_at=CURRENT_TIMESTAMP WHERE owner_user_id="+s.bind(1)+" AND session_key="+s.bind(2)+" AND status="+s.bind(3), ownerUserID, strings.TrimSpace(sessionKey), PairingStatusActive); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE im_pairing_sessions SET session_materialized=1, updated_at=CURRENT_TIMESTAMP WHERE owner_user_id="+s.bind(1)+" AND session_key="+s.bind(2), ownerUserID, strings.TrimSpace(sessionKey))
	return err
}
