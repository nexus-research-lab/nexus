// INPUT: owner/Agent 标识与 Agent 仓储提供的事务删除回调。
// OUTPUT: 锁内影响快照、删除后精确核验，以及受影响 Channel runtime 的立即注销。
// POS: Channels 对 Agent 删除消费侧协调接口的实现，不让 agent 反向依赖 channels。
package channels

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type agentDeletionImpact struct {
	controlVersion int64
	channelTypes   []string
	accounts       map[string][]string
	pairingIDs     []string
}

func (s *ControlService) CoordinateAgentDeletion(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	deletePersistent func(context.Context) error,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return errors.New("agent_id is required")
	}
	if deletePersistent == nil {
		return errors.New("Agent 持久删除回调不能为空")
	}

	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()

	channelTypes, err := s.agentDeletionChannelTypes(ctx, ownerUserID, agentID)
	if err != nil {
		return fmt.Errorf("读取 Agent Channel 删除影响: %w", err)
	}
	channelUnlocks := make([]func(), 0, len(channelTypes))
	for _, channelType := range channelTypes {
		channelUnlocks = append(channelUnlocks, s.lockChannelMutation(ownerUserID, channelType))
	}
	defer func() {
		for index := len(channelUnlocks) - 1; index >= 0; index-- {
			channelUnlocks[index]()
		}
	}()
	unlockPairings := s.lockPairingMutation(ownerUserID)
	defer unlockPairings()

	impact, err := s.loadAgentDeletionImpact(ctx, ownerUserID, agentID, channelTypes)
	if err != nil {
		return fmt.Errorf("确认 Agent Channel/account/pairing 删除影响: %w", err)
	}
	if err = deletePersistent(ctx); err != nil {
		return err
	}

	errs := make([]error, 0)
	if verifyErr := s.verifyAgentDeletionImpact(ctx, ownerUserID, agentID, impact); verifyErr != nil {
		errs = append(errs, verifyErr)
	}
	if s.router != nil {
		for _, channelType := range impact.channelTypes {
			if unregisterErr := s.router.UnregisterForOwner(ctx, ownerUserID, channelType); unregisterErr != nil {
				errs = append(errs, fmt.Errorf("停止 %s runtime: %w", channelType, unregisterErr))
			}
		}
	}
	return errors.Join(errs...)
}

func (s *ControlService) agentDeletionChannelTypes(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) ([]string, error) {
	query := `
SELECT DISTINCT channel_type
FROM im_channel_configs
WHERE owner_user_id = ` + s.bind(1) + ` AND agent_id = ` + s.bind(2)
	rows, err := s.db.QueryContext(ctx, query, ownerUserID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channelTypes := make([]string, 0)
	for rows.Next() {
		var channelType string
		if err = rows.Scan(&channelType); err != nil {
			return nil, err
		}
		channelTypes = append(channelTypes, normalizeIMChannelType(channelType))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	slices.Sort(channelTypes)
	return slices.Compact(channelTypes), nil
}

func (s *ControlService) loadAgentDeletionImpact(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	channelTypes []string,
) (agentDeletionImpact, error) {
	impact := agentDeletionImpact{
		channelTypes: slices.Clone(channelTypes),
		accounts:     make(map[string][]string, len(channelTypes)),
	}
	version, err := s.GetChannelControlVersion(ctx, ownerUserID)
	if err != nil {
		return agentDeletionImpact{}, err
	}
	impact.controlVersion = version
	for _, channelType := range channelTypes {
		rows, err := s.listChannelAccountRows(ctx, ownerUserID, channelType)
		if err != nil {
			return agentDeletionImpact{}, err
		}
		for _, row := range rows {
			impact.accounts[channelType] = append(impact.accounts[channelType], row.AccountID)
		}
	}
	query := `
SELECT pairing_id
FROM im_pairings
WHERE owner_user_id = ` + s.bind(1) + ` AND agent_id = ` + s.bind(2) + `
ORDER BY pairing_id`
	rows, err := s.db.QueryContext(ctx, query, ownerUserID, agentID)
	if err != nil {
		return agentDeletionImpact{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var pairingID string
		if err = rows.Scan(&pairingID); err != nil {
			return agentDeletionImpact{}, err
		}
		impact.pairingIDs = append(impact.pairingIDs, strings.TrimSpace(pairingID))
	}
	return impact, rows.Err()
}

func (s *ControlService) verifyAgentDeletionImpact(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	impact agentDeletionImpact,
) error {
	errs := make([]error, 0)
	version, err := s.GetChannelControlVersion(ctx, ownerUserID)
	if err != nil {
		errs = append(errs, err)
	} else if version != impact.controlVersion+1 {
		errs = append(errs, fmt.Errorf(
			"Agent %s 删除后的 Channel version 异常: before=%d after=%d",
			agentID,
			impact.controlVersion,
			version,
		))
	}
	for _, channelType := range impact.channelTypes {
		exists, err := s.HasChannelConfig(ctx, ownerUserID, channelType)
		if err != nil {
			errs = append(errs, err)
		} else if exists {
			errs = append(errs, fmt.Errorf("Agent %s 的 Channel %s 配置删除后仍存在", agentID, channelType))
		}
		for _, accountID := range impact.accounts[channelType] {
			exists, err = s.HasChannelAccount(ctx, ownerUserID, channelType, accountID)
			if err != nil {
				errs = append(errs, err)
			} else if exists {
				errs = append(errs, fmt.Errorf(
					"Agent %s 的 Channel %s account %s 删除后仍存在",
					agentID,
					channelType,
					accountID,
				))
			}
		}
	}
	for _, pairingID := range impact.pairingIDs {
		exists, err := s.HasPairing(ctx, ownerUserID, pairingID)
		if err != nil {
			errs = append(errs, err)
		} else if exists {
			errs = append(errs, fmt.Errorf("Agent %s 的 pairing %s 删除后仍存在", agentID, pairingID))
		}
	}
	return errors.Join(errs...)
}
