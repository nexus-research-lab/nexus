// INPUT: active pairing 与具体外部账号/聊天/话题目标。
// OUTPUT: 当前具体 IM Session key 及删除后的安全代次轮换。
// POS: wildcard pairing 与 concrete external Session identity 的持久化边界。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type pairingSessionTarget struct {
	OwnerUserID string
	PairingID   string
	ChannelType string
	AccountID   string
	ChatType    string
	ExternalRef string
	ThreadID    string
}

type pairingSessionRecord struct {
	Target              pairingSessionTarget
	SessionKey          string
	SessionMaterialized bool
}

func (s *ControlService) findPairingSession(ctx context.Context, target pairingSessionTarget) (*pairingSessionRecord, error) {
	query := `
SELECT pairing_id, session_key, session_materialized
FROM im_pairing_sessions
WHERE owner_user_id = ` + s.bind(1) + `
  AND channel_type = ` + s.bind(2) + `
  AND account_id = ` + s.bind(3) + `
  AND chat_type = ` + s.bind(4) + `
  AND external_ref = ` + s.bind(5) + `
  AND thread_id = ` + s.bind(6)
	var item pairingSessionRecord
	item.Target = target
	err := s.db.QueryRowContext(ctx, query,
		target.OwnerUserID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID,
	).Scan(&item.Target.PairingID, &item.SessionKey, &item.SessionMaterialized)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.SessionKey = strings.TrimSpace(item.SessionKey)
	return &item, nil
}

func (s *ControlService) findPairingSessionBySessionKey(ctx context.Context, ownerUserID, sessionKey string) (*pairingRow, error) {
	query := `
SELECT p.pairing_id, p.owner_user_id, p.channel_type, p.account_id, p.chat_type, p.external_ref, p.thread_id,
       p.external_name, p.agent_id, p.status, p.source, p.session_key, p.session_materialized,
       p.last_message_at, p.created_at, p.updated_at
FROM im_pairing_sessions ps
JOIN im_pairings p ON p.owner_user_id = ps.owner_user_id AND p.pairing_id = ps.pairing_id

WHERE ps.owner_user_id = ` + s.bind(1) + ` AND ps.session_key = ` + s.bind(2) + ` AND p.status = ` + s.bind(3) + `
`
	item, err := scanPairingScanner(s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(sessionKey), PairingStatusActive))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) upsertPairingSession(ctx context.Context, target pairingSessionTarget, sessionKey string) error {
	query := `
INSERT INTO im_pairing_sessions (owner_user_id, pairing_id, channel_type, account_id, chat_type, external_ref, thread_id, session_key)
VALUES (` + s.bind(1) + `,` + s.bind(2) + `,` + s.bind(3) + `,` + s.bind(4) + `,` + s.bind(5) + `,` + s.bind(6) + `,` + s.bind(7) + `,` + s.bind(8) + `)
ON CONFLICT (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, target.OwnerUserID, target.PairingID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID, strings.TrimSpace(sessionKey))
	return err
}

func (s *ControlService) updatePairingSession(ctx context.Context, target pairingSessionTarget, oldKey, newKey string, materialized bool) (string, error) {
	query := "UPDATE im_pairing_sessions SET session_key=" + s.bind(1) + ", session_materialized=" + s.bind(2) + ", updated_at=CURRENT_TIMESTAMP WHERE owner_user_id=" + s.bind(3) + " AND channel_type=" + s.bind(4) + " AND account_id=" + s.bind(5) + " AND chat_type=" + s.bind(6) + " AND external_ref=" + s.bind(7) + " AND thread_id=" + s.bind(8) + " AND session_key=" + s.bind(9)
	result, err := s.db.ExecContext(ctx, query, strings.TrimSpace(newKey), materialized, target.OwnerUserID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID, strings.TrimSpace(oldKey))
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 1 {
		return strings.TrimSpace(newKey), nil
	}
	current, err := s.findPairingSession(ctx, target)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", ErrPairingNotFound
	}
	return current.SessionKey, nil
}

func pairingSessionTargetFromIngress(ownerUserID string, request ingressPairingTarget) pairingSessionTarget {
	return pairingSessionTarget{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		ChannelType: normalizeIMChannelType(request.channelType),
		AccountID:   strings.TrimSpace(request.accountID),
		ChatType:    protocol.NormalizeSessionChatType(request.chatType),
		ExternalRef: strings.TrimSpace(request.externalRef),
		ThreadID:    strings.TrimSpace(request.threadID),
	}
}

func concretePairingSessionKey(row pairingRow, target pairingSessionTarget, generation string) string {
	return protocol.BuildAgentAccountSessionKeyWithGeneration(
		row.AgentID,
		target.ChannelType,
		target.ChatType,
		target.AccountID,
		target.ExternalRef,
		target.ThreadID,
		generation,
	)
}

func (s *ControlService) resolveConcretePairingSession(ctx context.Context, target pairingSessionTarget, pairing pairingRow) (string, error) {
	record, err := s.findPairingSession(ctx, target)
	if err != nil {
		return "", err
	}
	if record == nil {
		generation := protocol.ParseSessionKey(pairing.SessionKey).Generation
		key := concretePairingSessionKey(pairing, target, generation)
		if s.router != nil && s.router.sessionProjectionConfigured() {
			deleted, deletionErr := s.router.sessionDeleted(ctx, key)
			if deletionErr != nil {
				return "", deletionErr
			}
			if deleted {
				key = concretePairingSessionKey(pairing, target, s.idFactory("session"))
			}
		}
		if err = s.upsertPairingSession(ctx, target, key); err != nil {
			return "", err
		}
		record, err = s.findPairingSession(ctx, target)
		if err != nil {
			return "", err
		}
		if record == nil {
			return key, nil
		}
	}
	if s.router != nil && s.router.sessionProjectionConfigured() {
		var stored *protocol.Session
		if record.SessionMaterialized {
			stored, err = s.resolveDeliverySession(ctx, record.SessionKey)
			if err != nil {
				return "", err
			}
		}
		deleted, deletionErr := s.router.sessionDeleted(ctx, record.SessionKey)
		if deletionErr != nil {
			return "", deletionErr
		}
		if deleted || (record.SessionMaterialized && stored == nil) {
			newKey := concretePairingSessionKey(pairing, target, s.idFactory("session"))
			currentKey, updateErr := s.updatePairingSession(ctx, target, record.SessionKey, newKey, false)
			if updateErr != nil {
				return "", updateErr
			}
			if _, err = s.db.ExecContext(ctx, "UPDATE im_deliveries SET return_revoked=1 WHERE owner_user_id="+s.bind(1)+" AND pairing_id="+s.bind(2), target.OwnerUserID, target.PairingID); err != nil {
				return "", err
			}
			return currentKey, nil
		}
	}
	return record.SessionKey, nil
}
