// INPUT: owner 作用域的 pairing 查询、创建、字段更新与删除请求。
// OUTPUT: 保留 ingress 消息时间等非目标字段的 pairing 持久化结果。
// POS: Pairing 人工控制入口，与 ingress writer 共享 owner 级写锁。
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
	return s.createPairing(ctx, ownerUserID, request, 0)
}

func (s *ControlService) CreatePairingAtVersion(
	ctx context.Context,
	ownerUserID string,
	request CreatePairingRequest,
	expectedVersion int64,
) (*PairingView, error) {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return nil, err
	}
	return s.createPairing(ctx, ownerUserID, request, expectedVersion)
}

func (s *ControlService) createPairing(
	ctx context.Context,
	ownerUserID string,
	request CreatePairingRequest,
	expectedVersion int64,
) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	row, err := s.buildPairingRow(ctx, ownerUserID, request)
	if err != nil {
		if _, classified := ChannelControlMutationEffect(err); classified {
			return nil, err
		}
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	created, err := s.upsertPairingRowAndReloadAtVersion(ctx, row, expectedVersion)
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
	return s.updatePairing(ctx, ownerUserID, pairingID, request, 0)
}

func (s *ControlService) UpdatePairingAtVersion(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	request UpdatePairingRequest,
	expectedVersion int64,
) (*PairingView, error) {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return nil, err
	}
	return s.updatePairing(ctx, ownerUserID, pairingID, request, expectedVersion)
}

func (s *ControlService) updatePairing(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	request UpdatePairingRequest,
	expectedVersion int64,
) (*PairingView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	pairingID = strings.TrimSpace(pairingID)
	if request.AgentID != nil {
		agentID := strings.TrimSpace(*request.AgentID)
		if agentID == "" {
			return nil, invalidChannelControl(errors.New("agent_id cannot be empty"))
		}
		if err := s.ensureAgent(ctx, agentID); err != nil {
			return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
		}
		request.AgentID = &agentID
	}

	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockPairing := s.lockPairingMutation(ownerUserID)
	defer unlockPairing()

	var updatedRow *pairingRow
	_, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		existing, loadErr := s.getPairingRowFrom(ctx, tx, ownerUserID, pairingID)
		if loadErr != nil {
			return loadErr
		}
		if existing == nil {
			return ErrPairingNotFound
		}
		if request.Status != nil {
			status := normalizePairingStatus(*request.Status, existing.Status)
			if status == "" {
				return invalidChannelControl(errors.New("status is invalid"))
			}
			request.Status = &status
		}
		updatedRow, loadErr = s.patchPairingRowWith(ctx, tx, ownerUserID, pairingID, request)
		if loadErr != nil {
			return loadErr
		}
		if updatedRow == nil {
			return ErrPairingNotFound
		}
		return nil
	})
	if err != nil {
		return nil, channelControlVersionError(expectedVersion, err)
	}
	view := s.pairingView(ctx, *updatedRow)
	return &view, nil
}

func (s *ControlService) DeletePairing(ctx context.Context, ownerUserID string, pairingID string) error {
	return s.deletePairing(ctx, ownerUserID, pairingID, 0)
}

func (s *ControlService) DeletePairingAtVersion(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	expectedVersion int64,
) error {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return err
	}
	return s.deletePairing(ctx, ownerUserID, pairingID, expectedVersion)
}

func (s *ControlService) deletePairing(
	ctx context.Context,
	ownerUserID string,
	pairingID string,
	expectedVersion int64,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	pairingID = strings.TrimSpace(pairingID)
	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockPairing := s.lockPairingMutation(ownerUserID)
	defer unlockPairing()

	_, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		query := "DELETE FROM im_pairings WHERE owner_user_id = " + s.bind(1) + " AND pairing_id = " + s.bind(2)
		result, deleteErr := tx.ExecContext(ctx, query, ownerUserID, pairingID)
		if deleteErr != nil {
			return deleteErr
		}
		affected, deleteErr := result.RowsAffected()
		if deleteErr != nil {
			return deleteErr
		}
		if affected == 0 {
			return ErrPairingNotFound
		}
		return nil
	})
	if err != nil {
		return channelControlVersionError(expectedVersion, err)
	}
	return nil
}
