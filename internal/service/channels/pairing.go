package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
