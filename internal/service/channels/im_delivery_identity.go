// INPUT: Paired ingress and exact external Session identities.
// OUTPUT: Durable human input evidence and current pairing identity.
// POS: IM-only authorization adapter; never used by Agent private messaging.
package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

// IMDeliveryPairing returns the exact current grant after checking the real Session.
func (s *ControlService) IMDeliveryPairing(ctx context.Context, owner, agent, session string) (string, error) {
	if err := s.ValidateExternalSessionGrant(ctx, owner, agent, session); err != nil {
		return "", err
	}
	p := protocol.ParseSessionKey(session)
	if protocol.NormalizeSessionChatType(p.ChatType) != protocol.RoomTypeDM {
		return "", ErrExternalSessionGrantUnavailable
	}
	stored, err := s.resolveDeliverySession(ctx, session)
	if err != nil {
		return "", err
	}
	if stored == nil || stored.AgentID != agent {
		return "", ErrExternalSessionGrantUnavailable
	}
	row, err := s.findPairingBySessionKey(ctx, owner, session, PairingStatusActive)
	if row == nil && err == nil {
		row, err = s.findIngressPairingByTarget(ctx, owner, normalizeIMChannelType(p.Channel), p.AccountID, protocol.RoomTypeDM, p.Ref, ingressPairingThreadID(p.ChatType, p.ThreadID), PairingStatusActive)
	}
	if err != nil {
		return "", err
	}
	if row == nil || row.AgentID != agent {
		return "", ErrExternalSessionGrantUnavailable
	}
	return row.PairingID, nil
}

func (s *ControlService) recordDeliveryInput(ctx context.Context, r normalizedIngressRequest) error {
	p := r.parsed
	// Session materialization occurs in DM. Pairing has already been revalidated
	// by ingress; do not require an existing Session on its first human turn.
	row, err := s.findPairingBySessionKey(ctx, r.ownerUserID, r.sessionKey, PairingStatusActive)
	if row == nil && err == nil {
		row, err = s.findIngressPairingByTarget(ctx, r.ownerUserID, normalizeIMChannelType(p.Channel), p.AccountID, protocol.RoomTypeDM, p.Ref, ingressPairingThreadID(p.ChatType, p.ThreadID), PairingStatusActive)
		if err == nil && !fallbackPairingSessionMatches(row, p, r.sessionKey) {
			row = nil
		}
	}
	if err != nil {
		return err
	}
	if row == nil || row.AgentID != r.agentID {
		return ErrExternalSessionGrantUnavailable
	}
	input := imdelivery.Input{ID: r.reqID, OwnerUserID: r.ownerUserID, AgentID: r.agentID, SessionKey: r.sessionKey, RoundID: r.roundID, PairingID: row.PairingID, Content: r.content, Sender: p.Ref}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE im_ingress_messages SET delivery_input_json=`+s.bind(1)+` WHERE owner_user_id=`+s.bind(2)+` AND session_key=`+s.bind(3)+` AND round_id=`+s.bind(4)+` AND req_id=`+s.bind(5)+` AND (delivery_input_json='' OR delivery_input_json=`+s.bind(6)+`)`, string(raw), r.ownerUserID, r.sessionKey, r.roundID, r.reqID, string(raw))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("IM input evidence conflict")
	}
	return nil
}

// IMDeliveryInput requires exact ingress round and actual dispatched text. A
// queued payload edited in the workspace cannot acquire this human authority.
func (s *ControlService) IMDeliveryInput(ctx context.Context, owner, agent, session, round, content string) (imdelivery.Input, error) {
	if strings.TrimSpace(content) == "" {
		return imdelivery.Input{}, errors.New("IM 回传缺少当前人类输入")
	}
	return s.readDeliveryInput(ctx, owner, agent, session, "round_id", round, content)
}
func (s *ControlService) IMDeliveryContentInput(ctx context.Context, owner, agent, session, id string) (imdelivery.Input, error) {
	return s.readDeliveryInput(ctx, owner, agent, session, "req_id", id, "")
}
func (s *ControlService) readDeliveryInput(ctx context.Context, owner, agent, session, column, value, content string) (imdelivery.Input, error) {
	if column != "round_id" && column != "req_id" {
		return imdelivery.Input{}, imdelivery.ErrUnavailable
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT delivery_input_json FROM im_ingress_messages WHERE owner_user_id=`+s.bind(1)+` AND session_key=`+s.bind(2)+` AND agent_id=`+s.bind(3)+` AND `+column+`=`+s.bind(4)+` AND status IN ('processing','accepted')`, owner, session, agent, value).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return imdelivery.Input{}, errors.New("回传需要当前 IM 人类消息")
	}
	if err != nil {
		return imdelivery.Input{}, err
	}
	var input imdelivery.Input
	if json.Unmarshal([]byte(raw), &input) != nil || input.ID == "" || input.OwnerUserID != owner || input.SessionKey != session || input.AgentID != agent || (content != "" && input.Content != strings.TrimSpace(content)) {
		return input, errors.New("IM 人类消息身份不匹配")
	}
	pairing, err := s.IMDeliveryPairing(ctx, owner, agent, session)
	if err != nil {
		return input, err
	}
	if pairing != input.PairingID {
		return input, ErrExternalSessionGrantUnavailable
	}
	return input, nil
}
