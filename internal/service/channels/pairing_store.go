package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *ControlService) listPairingRows(ctx context.Context, ownerUserID string, query PairingQuery) ([]pairingRow, error) {
	sqlText := `
	SELECT pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1)
	args := []any{strings.TrimSpace(ownerUserID)}
	if channelType := normalizeIMChannelType(query.ChannelType); channelType != "" {
		args = append(args, channelType)
		sqlText += " AND channel_type = " + s.bind(len(args))
	}
	if status := normalizePairingStatus(query.Status, ""); status != "" {
		args = append(args, status)
		sqlText += " AND status = " + s.bind(len(args))
	}
	if agentID := strings.TrimSpace(query.AgentID); agentID != "" {
		args = append(args, agentID)
		sqlText += " AND agent_id = " + s.bind(len(args))
	}
	sqlText += " ORDER BY updated_at DESC, created_at DESC, pairing_id DESC"

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []pairingRow{}
	for rows.Next() {
		item, err := scanPairingScanner(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (s *ControlService) getPairingRow(ctx context.Context, ownerUserID string, pairingID string) (*pairingRow, error) {
	query := `
	SELECT pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + " AND pairing_id = " + s.bind(2)
	item, err := scanPairingScanner(s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(pairingID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) findPairingByTarget(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
	chatType string,
	externalRef string,
	threadID string,
	status string,
) (*pairingRow, error) {
	query := `
	SELECT pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	       agent_id, status, source, last_message_at, created_at, updated_at
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + `
	  AND channel_type = ` + s.bind(2) + `
	  AND account_id = ` + s.bind(3) + `
	  AND chat_type = ` + s.bind(4) + `
	  AND external_ref = ` + s.bind(5) + `
	  AND thread_id = ` + s.bind(6) + `
	  AND status = ` + s.bind(7) + `
	LIMIT 1`
	item, err := scanPairingScanner(s.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		normalizeIMChannelType(channelType),
		strings.TrimSpace(accountID),
		protocol.NormalizeSessionChatType(chatType),
		strings.TrimSpace(externalRef),
		strings.TrimSpace(threadID),
		normalizePairingStatus(status, PairingStatusActive),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s *ControlService) upsertPairingRowAndReload(ctx context.Context, row pairingRow) (*pairingRow, error) {
	if err := s.upsertPairingRow(ctx, row); err != nil {
		return nil, err
	}
	created, err := s.findPairingByTarget(
		ctx,
		row.OwnerUserID,
		row.ChannelType,
		row.AccountID,
		row.ChatType,
		row.ExternalRef,
		row.ThreadID,
		row.Status,
	)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrPairingNotFound
	}
	return created, nil
}

func (s *ControlService) upsertPairingRow(ctx context.Context, row pairingRow) error {
	if strings.TrimSpace(row.PairingID) == "" {
		row.PairingID = s.idFactory("pair")
	}
	if s.driver == "pgx" {
		query := `
	INSERT INTO im_pairings (
	    pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	    agent_id, status, source, last_message_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	ON CONFLICT (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id) DO UPDATE SET
    external_name = EXCLUDED.external_name,
    agent_id = EXCLUDED.agent_id,
    status = EXCLUDED.status,
    source = EXCLUDED.source,
    last_message_at = COALESCE(EXCLUDED.last_message_at, im_pairings.last_message_at),
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(
			ctx,
			query,
			row.PairingID,
			row.OwnerUserID,
			row.ChannelType,
			row.AccountID,
			row.ChatType,
			row.ExternalRef,
			row.ThreadID,
			nullStringValueOrNil(row.ExternalName),
			row.AgentID,
			row.Status,
			row.Source,
			nullTimeValueOrNil(row.LastMessageAt),
		)
		return err
	}
	query := `
	INSERT INTO im_pairings (
	    pairing_id, owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id, external_name,
	    agent_id, status, source, last_message_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id) DO UPDATE SET
    external_name = excluded.external_name,
    agent_id = excluded.agent_id,
    status = excluded.status,
    source = excluded.source,
    last_message_at = COALESCE(excluded.last_message_at, im_pairings.last_message_at),
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(
		ctx,
		query,
		row.PairingID,
		row.OwnerUserID,
		row.ChannelType,
		row.AccountID,
		row.ChatType,
		row.ExternalRef,
		row.ThreadID,
		nullStringValueOrNil(row.ExternalName),
		row.AgentID,
		row.Status,
		row.Source,
		nullTimeValueOrNil(row.LastMessageAt),
	)
	return err
}

func scanPairingScanner(row sqlScanner) (*pairingRow, error) {
	var item pairingRow
	err := row.Scan(
		&item.PairingID,
		&item.OwnerUserID,
		&item.ChannelType,
		&item.AccountID,
		&item.ChatType,
		&item.ExternalRef,
		&item.ThreadID,
		&item.ExternalName,
		&item.AgentID,
		&item.Status,
		&item.Source,
		&item.LastMessageAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.ChannelType = normalizeIMChannelType(item.ChannelType)
	item.AccountID = strings.TrimSpace(item.AccountID)
	item.ChatType = protocol.NormalizeSessionChatType(item.ChatType)
	return &item, nil
}
