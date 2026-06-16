package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *ControlService) ListPairings(ctx context.Context, ownerUserID string, query PairingQuery) ([]PairingView, error) {
	rows, err := s.listPairingRows(ctx, normalizeChannelOwnerUserID(ownerUserID), query)
	if err != nil {
		return nil, err
	}
	result := make([]PairingView, 0, len(rows))
	for _, row := range rows {
		result = append(result, s.pairingView(ctx, row))
	}
	return result, nil
}

func (s *ControlService) CreatePairing(ctx context.Context, ownerUserID string, request CreatePairingRequest) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	row, err := s.buildPairingRow(ctx, ownerUserID, request)
	if err != nil {
		return nil, err
	}
	created, err := s.upsertPairingRowAndReload(ctx, row)
	if err != nil {
		return nil, err
	}
	view := s.pairingView(ctx, *created)
	return &view, nil
}

func (s *ControlService) UpdatePairing(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	request UpdatePairingRequest,
) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	existing, err := s.getPairingRow(ctx, ownerUserID, strings.TrimSpace(pairingID))
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrPairingNotFound
	}
	if request.AgentID != nil {
		agentID := strings.TrimSpace(*request.AgentID)
		if agentID == "" {
			return nil, errors.New("agent_id cannot be empty")
		}
		if err = s.ensureAgent(ctx, agentID); err != nil {
			return nil, err
		}
		existing.AgentID = agentID
	}
	if request.Status != nil {
		status := normalizePairingStatus(*request.Status, existing.Status)
		if status == "" {
			return nil, errors.New("status is invalid")
		}
		existing.Status = status
	}
	if request.ExternalName != nil {
		existing.ExternalName = sql.NullString{
			String: strings.TrimSpace(*request.ExternalName),
			Valid:  strings.TrimSpace(*request.ExternalName) != "",
		}
	}
	if err = s.upsertPairingRow(ctx, *existing); err != nil {
		return nil, err
	}
	updated, err := s.getPairingRow(ctx, ownerUserID, existing.PairingID)
	if err != nil {
		return nil, err
	}
	view := s.pairingView(ctx, *updated)
	return &view, nil
}

func (s *ControlService) DeletePairing(ctx context.Context, ownerUserID string, pairingID string) error {
	query := "DELETE FROM im_pairings WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	result, err := s.db.ExecContext(ctx, query, normalizeChannelOwnerUserID(ownerUserID), strings.TrimSpace(pairingID))
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrPairingNotFound
	}
	return nil
}

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

func (s *ControlService) buildPairingRow(ctx context.Context, ownerUserID string, request CreatePairingRequest) (pairingRow, error) {
	channelType := normalizeIMChannelType(request.ChannelType)
	if _, ok := channelCatalogByType(channelType); !ok {
		return pairingRow{}, ErrChannelNotFound
	}
	chatType := protocol.NormalizeSessionChatType(request.ChatType)
	externalRef := strings.TrimSpace(request.ExternalRef)
	if externalRef == "" {
		return pairingRow{}, errors.New("external_ref is required")
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return pairingRow{}, errors.New("agent_id is required")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return pairingRow{}, err
	}
	status := normalizePairingStatus(request.Status, PairingStatusActive)
	if status == "" {
		return pairingRow{}, errors.New("status is invalid")
	}
	source := normalizePairingSource(request.Source, PairingSourceManual)
	if source == "" {
		return pairingRow{}, errors.New("source is invalid")
	}
	return pairingRow{
		PairingID:    s.idFactory("pair"),
		OwnerUserID:  strings.TrimSpace(ownerUserID),
		ChannelType:  channelType,
		AccountID:    strings.TrimSpace(request.AccountID),
		ChatType:     chatType,
		ExternalRef:  externalRef,
		ThreadID:     strings.TrimSpace(request.ThreadID),
		ExternalName: sql.NullString{String: strings.TrimSpace(request.ExternalName), Valid: strings.TrimSpace(request.ExternalName) != ""},
		AgentID:      agentID,
		Status:       status,
		Source:       source,
	}, nil
}

func (s *ControlService) pairingView(ctx context.Context, row pairingRow) PairingView {
	var lastMessageAt *time.Time
	if row.LastMessageAt.Valid {
		value := row.LastMessageAt.Time
		lastMessageAt = &value
	}
	return PairingView{
		PairingID:     row.PairingID,
		ChannelType:   row.ChannelType,
		AccountID:     row.AccountID,
		ChatType:      row.ChatType,
		ExternalRef:   row.ExternalRef,
		ThreadID:      row.ThreadID,
		SessionKey:    protocol.BuildAgentAccountSessionKey(row.AgentID, protocol.NormalizeSessionKeyChannelSegment(row.ChannelType), row.ChatType, row.AccountID, row.ExternalRef, row.ThreadID),
		ExternalName:  nullStringValue(row.ExternalName),
		AgentID:       row.AgentID,
		AgentName:     s.agentName(ctx, row.AgentID),
		Status:        row.Status,
		Source:        row.Source,
		LastMessageAt: lastMessageAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func (s *ControlService) channelStats(ctx context.Context, ownerUserID string) (map[string]ChannelStats, error) {
	query := `
SELECT channel_type, chat_type, status, COUNT(1)
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + `
GROUP BY channel_type, chat_type, status`
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]ChannelStats{}
	for rows.Next() {
		var channelType, chatType, status string
		var count int
		if err = rows.Scan(&channelType, &chatType, &status, &count); err != nil {
			return nil, err
		}
		channelType = normalizeIMChannelType(channelType)
		item := result[channelType]
		if status == PairingStatusPending {
			item.PendingCount += count
		}
		if status == PairingStatusActive && protocol.NormalizeSessionChatType(chatType) == "dm" {
			item.PairedUserCount += count
		}
		if status == PairingStatusActive && protocol.NormalizeSessionChatType(chatType) == "group" {
			item.PairedGroupCount += count
		}
		result[channelType] = item
	}
	return result, rows.Err()
}

func (s *ControlService) touchPairing(ctx context.Context, ownerUserID string, pairingID string) error {
	query := "UPDATE im_pairings SET last_message_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(pairingID))
	return err
}
