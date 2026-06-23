package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
