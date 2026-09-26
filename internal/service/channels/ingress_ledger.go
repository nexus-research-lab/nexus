package channels

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	ingressMessageStatusProcessing = "processing"
	ingressMessageStatusAccepted   = "accepted"
)

type ingressMessageClaimInput struct {
	BindingVersion int64
	Content        string
	OwnerUserID    string
	Channel        string
	AccountID      string
	ReqID          string
	AgentID        string
	SessionKey     string
	RoundID        string
}

type ingressMessageFinishInput struct {
	OwnerUserID  string
	Channel      string
	AccountID    string
	ReqID        string
	Status       string
	ErrorMessage *string
}

type ingressMessageRow struct {
	BindingVersion int64
	PayloadHash    string
	DispatchPhase  string
	OwnerUserID    string
	Channel        string
	AccountID      string
	ReqID          string
	AgentID        string
	SessionKey     string
	RoundID        string
	Status         string
	ErrorMessage   sql.NullString
}

func (s *ControlService) claimIngressMessage(ctx context.Context, input ingressMessageClaimInput) (bool, *IngressResult, error) {
	normalized := input.normalized()
	if normalized.OwnerUserID == "" || normalized.Channel == "" || normalized.ReqID == "" {
		return true, nil, nil
	}
	inserted, err := s.insertIngressMessageClaim(ctx, normalized)
	if err != nil {
		return false, nil, err
	}
	if inserted {
		return true, nil, nil
	}
	row, err := s.getIngressMessage(ctx, normalized.OwnerUserID, normalized.Channel, normalized.AccountID, normalized.ReqID)
	if err != nil {
		return false, nil, err
	}
	if row == nil {
		return true, nil, nil
	}
	if row.PayloadHash != "" && row.PayloadHash != fmt.Sprintf("%x", sha256.Sum256([]byte(normalized.Content))) {
		return false, nil, fmt.Errorf("通道消息身份对应不同正文")
	}
	result := ingressResultFromMessageRow(*row)
	if row.Status == ingressMessageStatusAccepted {
		return false, result, nil
	}
	if row.Status == ingressMessageStatusProcessing && row.DispatchPhase == "prepared" {
		if row.AgentID != normalized.AgentID || row.SessionKey != normalized.SessionKey || row.BindingVersion != normalized.BindingVersion {
			return false, result, ErrIngressOutcomeUnknown
		}
		return true, result, nil
	}
	return false, result, ErrIngressOutcomeUnknown
}

func (s *ControlService) finishIngressMessage(ctx context.Context, input ingressMessageFinishInput) error {
	normalized := input.normalized()
	if normalized.OwnerUserID == "" || normalized.Channel == "" || normalized.ReqID == "" {
		return nil
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = ingressMessageStatusAccepted
	}
	errorMessage := ""
	if normalized.ErrorMessage != nil {
		errorMessage = *normalized.ErrorMessage
	}
	query := fmt.Sprintf(
		`UPDATE im_ingress_messages
SET status = %s,
    error_message = %s,
    completed_at = CASE WHEN %s IN ('accepted', 'failed') THEN CURRENT_TIMESTAMP ELSE completed_at END,
    updated_at = CURRENT_TIMESTAMP
	WHERE owner_user_id = %s AND channel_type = %s AND account_id = %s AND req_id = %s`,
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
		s.bind(5),
		s.bind(6),
		s.bind(7),
	)
	_, err := s.db.ExecContext(
		ctx,
		query,
		status,
		nullableString(errorMessage),
		status,
		normalized.OwnerUserID,
		normalized.Channel,
		normalized.AccountID,
		normalized.ReqID,
	)
	return err
}

func (s *ControlService) insertIngressMessageClaim(ctx context.Context, input ingressMessageClaimInput) (bool, error) {
	query := fmt.Sprintf(
		`INSERT INTO im_ingress_messages (
	    owner_user_id,
	    channel_type,
	    account_id,
	    req_id,
    agent_id,
    session_key,
    round_id,
    status,
    payload_hash,
    binding_version,
    dispatch_phase,
    created_at,
    updated_at
	) VALUES (%s, 'prepared', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(owner_user_id, channel_type, account_id, req_id) DO NOTHING`,
		s.bindList(10),
	)
	result, err := s.db.ExecContext(
		ctx,
		query,
		input.OwnerUserID,
		input.Channel,
		input.AccountID,
		input.ReqID,
		input.AgentID,
		input.SessionKey,
		input.RoundID,
		ingressMessageStatusProcessing,
		fmt.Sprintf("%x", sha256.Sum256([]byte(input.Content))),
		input.BindingVersion,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *ControlService) getIngressMessage(ctx context.Context, ownerUserID string, channel string, accountID string, reqID string) (*ingressMessageRow, error) {
	query := fmt.Sprintf(
		`SELECT owner_user_id, channel_type, account_id, req_id, agent_id, session_key, round_id, status, error_message, dispatch_phase, payload_hash, binding_version
	FROM im_ingress_messages
	WHERE owner_user_id = %s AND channel_type = %s AND account_id = %s AND req_id = %s`,
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
	)
	var row ingressMessageRow
	err := s.db.QueryRowContext(ctx, query, ownerUserID, channel, strings.TrimSpace(accountID), reqID).Scan(
		&row.OwnerUserID,
		&row.Channel,
		&row.AccountID,
		&row.ReqID,
		&row.AgentID,
		&row.SessionKey,
		&row.RoundID,
		&row.Status,
		&row.ErrorMessage,
		&row.DispatchPhase,
		&row.PayloadHash,
		&row.BindingVersion,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ingressResultFromMessageRow(row ingressMessageRow) *IngressResult {
	return &IngressResult{
		Channel:    protocol.NormalizeStoredChannelType(row.Channel),
		AgentID:    strings.TrimSpace(row.AgentID),
		SessionKey: strings.TrimSpace(row.SessionKey),
		RoundID:    strings.TrimSpace(row.RoundID),
		ReqID:      strings.TrimSpace(row.ReqID),
		Duplicate:  true,
	}
}

func (input ingressMessageClaimInput) normalized() ingressMessageClaimInput {
	input.OwnerUserID = normalizeChannelOwnerUserID(input.OwnerUserID)
	input.Channel = normalizeIMChannelType(input.Channel)
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.ReqID = strings.TrimSpace(input.ReqID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.SessionKey = strings.TrimSpace(input.SessionKey)
	input.RoundID = strings.TrimSpace(input.RoundID)
	return input
}

func (input ingressMessageFinishInput) normalized() ingressMessageFinishInput {
	input.OwnerUserID = normalizeChannelOwnerUserID(input.OwnerUserID)
	input.Channel = normalizeIMChannelType(input.Channel)
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.ReqID = strings.TrimSpace(input.ReqID)
	if input.ErrorMessage != nil {
		value := strings.TrimSpace(*input.ErrorMessage)
		input.ErrorMessage = &value
	}
	return input
}

// beginIngressDispatch 是副作用前的唯一 CAS；准备阶段可重复，越过此处只能核验。
func (s *ControlService) beginIngressDispatch(ctx context.Context, request normalizedIngressRequest) error {
	query := fmt.Sprintf(`UPDATE im_ingress_messages SET dispatch_phase='dispatching', updated_at=CURRENT_TIMESTAMP
 WHERE owner_user_id=%s AND channel_type=%s AND account_id=%s AND req_id=%s AND status='processing' AND dispatch_phase='prepared'`, s.bind(1), s.bind(2), s.bind(3), s.bind(4))
	result, err := s.db.ExecContext(ctx, query, request.ownerUserID, request.channelStored, request.accountID, request.reqID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrIngressOutcomeUnknown
	}
	return nil
}
