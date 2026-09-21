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
  AND pairing_id = ` + s.bind(2) + `
  AND channel_type = ` + s.bind(3) + `
  AND account_id = ` + s.bind(4) + `
  AND chat_type = ` + s.bind(5) + `
  AND external_ref = ` + s.bind(6) + `
  AND thread_id = ` + s.bind(7)
	var item pairingSessionRecord
	item.Target = target
	err := s.db.QueryRowContext(ctx, query,
		target.OwnerUserID, target.PairingID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID,
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

// findPairingSessionAnyTarget is used only while repairing a target whose
// pairing identity changed. The normal read path above deliberately includes
// pairing_id in its predicate so an old mapping can never be returned as the
// current pairing's Session.
func (s *ControlService) findPairingSessionAnyTarget(ctx context.Context, target pairingSessionTarget) (*pairingSessionRecord, error) {
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
       p.external_name, p.agent_id, p.status, p.source, ps.session_key, ps.session_materialized,
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
	if strings.TrimSpace(target.PairingID) == "" {
		return errors.New("pairing_id is required")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return errors.New("session_key is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// A deleted/recreated pairing can reuse the same external target. Revoke
	// every delivery that points at the displaced concrete key before replacing
	// the projection, including deliveries whose old pairing row is already
	// gone.
	revokeQuery := `
UPDATE im_deliveries
SET return_revoked = 1
WHERE owner_user_id = ` + s.bind(1) + `
  AND target_session_key IN (
      SELECT session_key
      FROM im_pairing_sessions
      WHERE owner_user_id = ` + s.bind(2) + `
        AND channel_type = ` + s.bind(3) + `
        AND account_id = ` + s.bind(4) + `
        AND chat_type = ` + s.bind(5) + `
        AND external_ref = ` + s.bind(6) + `
        AND thread_id = ` + s.bind(7) + `
  )
  AND target_session_key <> ` + s.bind(8)
	if _, err = tx.ExecContext(ctx, revokeQuery,
		target.OwnerUserID, target.OwnerUserID, target.ChannelType, target.AccountID,
		target.ChatType, target.ExternalRef, target.ThreadID, sessionKey,
	); err != nil {
		return err
	}
	routeQuery := `
DELETE FROM automation_delivery_routes
WHERE (
      (COALESCE(session_key, '') <> '' AND session_key IN (
          SELECT session_key
          FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(1) + `
            AND channel_type = ` + s.bind(2) + `
            AND account_id = ` + s.bind(3) + `
            AND chat_type = ` + s.bind(4) + `
            AND external_ref = ` + s.bind(5) + `
            AND thread_id = ` + s.bind(6) + `
      ))
      OR (COALESCE(session_key, '') = ''
          AND COALESCE(channel, '') = ` + s.bind(7) + `
          AND COALESCE("to", '') = ` + s.bind(8) + `
          AND COALESCE(account_id, '') = ` + s.bind(9) + `
          AND COALESCE(thread_id, '') = ` + s.bind(10) + `)
  )
	AND COALESCE(session_key, '') <> ` + s.bind(11)
	if _, err = tx.ExecContext(ctx, routeQuery,
		target.OwnerUserID, target.ChannelType, target.AccountID,
		target.ChatType, target.ExternalRef, target.ThreadID, target.ChannelType,
		target.ExternalRef, target.AccountID, target.ThreadID, sessionKey,
	); err != nil {
		return err
	}

	query := `
INSERT INTO im_pairing_sessions (owner_user_id, pairing_id, channel_type, account_id, chat_type, external_ref, thread_id, session_key, session_materialized)
VALUES (` + s.bind(1) + `,` + s.bind(2) + `,` + s.bind(3) + `,` + s.bind(4) + `,` + s.bind(5) + `,` + s.bind(6) + `,` + s.bind(7) + `,` + s.bind(8) + `,` + s.bind(9) + `)
ON CONFLICT (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id) DO UPDATE SET
    pairing_id = excluded.pairing_id,
    session_key = excluded.session_key,
    session_materialized = excluded.session_materialized,
    updated_at = CURRENT_TIMESTAMP`
	if _, err = tx.ExecContext(ctx, query, target.OwnerUserID, target.PairingID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID, sessionKey, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ControlService) updatePairingSession(ctx context.Context, target pairingSessionTarget, oldKey, newKey string, materialized bool) (string, error) {
	query := "UPDATE im_pairing_sessions SET session_key=" + s.bind(1) + ", session_materialized=" + s.bind(2) + ", updated_at=CURRENT_TIMESTAMP WHERE owner_user_id=" + s.bind(3) + " AND pairing_id=" + s.bind(4) + " AND channel_type=" + s.bind(5) + " AND account_id=" + s.bind(6) + " AND chat_type=" + s.bind(7) + " AND external_ref=" + s.bind(8) + " AND thread_id=" + s.bind(9) + " AND session_key=" + s.bind(10)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, query, strings.TrimSpace(newKey), materialized, target.OwnerUserID, target.PairingID, target.ChannelType, target.AccountID, target.ChatType, target.ExternalRef, target.ThreadID, strings.TrimSpace(oldKey))
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 1 {
		// The old concrete key is a return-address capability. Revoke its
		// deliveries in the same transaction as the mapping rotation so a crash
		// cannot leave an apparently valid grant pointing at a deleted Session.
		revokeQuery := "UPDATE im_deliveries SET return_revoked=1 WHERE owner_user_id=" + s.bind(1) + " AND (pairing_id=" + s.bind(2) + " OR target_session_key=" + s.bind(3) + ")"
		if _, err = tx.ExecContext(ctx, revokeQuery, target.OwnerUserID, target.PairingID, strings.TrimSpace(oldKey)); err != nil {
			return "", err
		}
		if err = tx.Commit(); err != nil {
			return "", err
		}
		return strings.TrimSpace(newKey), nil
	}
	if err = tx.Rollback(); err != nil {
		return "", err
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
	// Concrete projections are a single logical target. Serialize their
	// read/repair/write cycle so two simultaneous first messages cannot each
	// return a different key while the last upsert wins in the database.
	unlockPairing := s.lockPairingMutation(target.OwnerUserID)
	defer unlockPairing()
	// ResolveIngressSession read the parent before taking this lock. A pairing
	// delete/revoke can therefore commit in between that read and the concrete
	// projection write. Re-read the exact parent while serialized so a stale
	// ingress cannot recreate an orphan projection for a deleted pairing.
	currentPairing, err := s.getPairingRow(ctx, target.OwnerUserID, pairing.PairingID)
	if err != nil {
		return "", err
	}
	if currentPairing == nil || currentPairing.Status != PairingStatusActive ||
		strings.TrimSpace(currentPairing.AgentID) != strings.TrimSpace(pairing.AgentID) {
		return "", ErrExternalSessionGrantUnavailable
	}
	pairing = *currentPairing

	record, err := s.findPairingSession(ctx, target)
	if err != nil {
		return "", err
	}
	if record == nil {
		stale, staleErr := s.findPairingSessionAnyTarget(ctx, target)
		if staleErr != nil {
			return "", staleErr
		}
		generation := protocol.ParseSessionKey(pairing.SessionKey).Generation
		if generation == "" || (stale != nil && stale.Target.PairingID != target.PairingID) {
			// The external target was rebound or recreated. Do not let the new
			// pairing inherit the old generation-less key. Wildcard projections
			// also always receive their own generation, even when the parent
			// pairing predates generation support.
			generation = s.idFactory("session")
		}
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
			return currentKey, nil
		}
	}
	return record.SessionKey, nil
}
