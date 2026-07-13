package channels

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	ingressMessageStatusProcessing = "processing"
	ingressMessageStatusAccepted   = "accepted"
	ingressMessageStatusFailed     = "failed"
)

type ingressMessageClaimInput struct {
	OwnerUserID string
	Channel     string
	AccountID   string
	ReqID       string
	AgentID     string
	SessionKey  string
	RoundID     string
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
	OwnerUserID  string
	Channel      string
	AccountID    string
	ReqID        string
	AgentID      string
	SessionKey   string
	RoundID      string
	Status       string
	ErrorMessage sql.NullString
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
	reclaimed, err := s.reclaimFailedIngressMessage(ctx, normalized)
	if err != nil {
		return false, nil, err
	}
	if reclaimed {
		return true, nil, nil
	}
	row, err := s.getIngressMessage(ctx, normalized.OwnerUserID, normalized.Channel, normalized.AccountID, normalized.ReqID)
	if err != nil {
		return false, nil, err
	}
	if row == nil {
		return true, nil, nil
	}
	return false, ingressResultFromMessageRow(*row), nil
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
    created_at,
    updated_at
	) VALUES (%s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(owner_user_id, channel_type, account_id, req_id) DO NOTHING`,
		s.bindList(8),
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

func (s *ControlService) reclaimFailedIngressMessage(ctx context.Context, input ingressMessageClaimInput) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE im_ingress_messages
SET agent_id = %s,
    session_key = %s,
    round_id = %s,
    status = %s,
    error_message = NULL,
    completed_at = NULL,
    updated_at = CURRENT_TIMESTAMP
	WHERE owner_user_id = %s
	  AND channel_type = %s
	  AND account_id = %s
	  AND req_id = %s
	  AND status = %s`,
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
		s.bind(5),
		s.bind(6),
		s.bind(7),
		s.bind(8),
		s.bind(9),
	)
	result, err := s.db.ExecContext(
		ctx,
		query,
		input.AgentID,
		input.SessionKey,
		input.RoundID,
		ingressMessageStatusProcessing,
		input.OwnerUserID,
		input.Channel,
		input.AccountID,
		input.ReqID,
		ingressMessageStatusFailed,
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
		`SELECT owner_user_id, channel_type, account_id, req_id, agent_id, session_key, round_id, status, error_message
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
