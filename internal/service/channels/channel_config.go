// INPUT: owner 作用域的渠道配置、凭据与账号变更请求。
// OUTPUT: 事务化持久化结果，以及启动成功后才替换的热重载 runtime。
// POS: Channels 配置写入口，负责同一 owner+channel 的串行化与运行态错误传播。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (s *ControlService) ListChannels(ctx context.Context, ownerUserID string) ([]ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	rows, err := s.listChannelConfigRows(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	stats, err := s.channelStats(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	accountRows, err := s.channelAccountsByType(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	byType := make(map[string]channelConfigRow, len(rows))
	for _, row := range rows {
		byType[row.ChannelType] = row
	}

	result := make([]ChannelConfigView, 0, len(channelCatalog()))
	for _, catalog := range channelCatalog() {
		catalogStats := stats[catalog.ChannelType]
		if isPlannedChannel(catalog.ChannelType) {
			catalogStats = ChannelStats{}
		}
		view := ChannelConfigView{
			ChannelCatalogItem: catalog,
			ConnectionState:    "not_configured",
			Status:             "not_configured",
			Stats:              catalogStats,
		}
		row, ok := byType[catalog.ChannelType]
		if ok {
			publicConfig, _ := decodeStringMap(row.ConfigJSON)
			publicConfig = publicChannelConfigForView(catalog.ChannelType, publicConfig)
			view.Configured = true
			view.Status = firstNonEmpty(row.Status, ChannelConfigStatusConfigured)
			view.ConnectionState = s.connectionStateFor(ownerUserID, catalog.ChannelType, view.Status)
			view.AgentID = row.AgentID
			view.AgentName = s.agentName(ctx, row.AgentID)
			view.PublicConfig = publicConfig
			view.HasCredentials = row.CredentialsEncrypted.Valid && strings.TrimSpace(row.CredentialsEncrypted.String) != ""
			if accounts := channelAccountViews(accountRows[catalog.ChannelType]); len(accounts) > 0 {
				view.Accounts = accounts
			}
			if catalog.ChannelType == ChannelTypeWeixinPersonal && len(view.Accounts) > 0 {
				view.HasCredentials = true
				publicConfig["account_count"] = fmt.Sprintf("%d", len(view.Accounts))
			}
			view.LastError = nullStringValue(row.LastError)
			view.QRPayload = publicConfig["qr_payload"]
			view.UpdatedAt = &row.UpdatedAt
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *ControlService) CountConfiguredChannels(ctx context.Context, ownerUserID string) (int, error) {
	rows, err := s.listChannelConfigRows(ctx, normalizeChannelOwnerUserID(ownerUserID))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled || isPlannedChannel(row.ChannelType) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *ControlService) CountConnectedChannels(ctx context.Context, ownerUserID string) (int, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	rows, err := s.listChannelConfigRows(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	accountRows, err := s.channelAccountsByType(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled || isPlannedChannel(row.ChannelType) {
			continue
		}
		if row.Status == ChannelConfigStatusConnected ||
			slices.ContainsFunc(accountRows[row.ChannelType], func(account channelAccountRow) bool {
				return account.Status == ChannelConfigStatusConnected
			}) {
			count++
		}
	}
	return count, nil
}

func (s *ControlService) CountActivePairings(ctx context.Context, ownerUserID string) (int, error) {
	rows, err := s.listPairingRows(ctx, normalizeChannelOwnerUserID(ownerUserID), PairingQuery{Status: PairingStatusActive})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if isPlannedChannel(row.ChannelType) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *ControlService) UpsertChannelConfig(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	request UpsertChannelConfigRequest,
) (*ChannelConfigView, error) {
	return s.upsertChannelConfig(ctx, ownerUserID, channelType, request, 0)
}

func (s *ControlService) UpsertChannelConfigAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	request UpsertChannelConfigRequest,
	expectedVersion int64,
) (*ChannelConfigView, error) {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return nil, err
	}
	return s.upsertChannelConfig(ctx, ownerUserID, channelType, request, expectedVersion)
}

func (s *ControlService) upsertChannelConfig(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	request UpsertChannelConfigRequest,
	expectedVersion int64,
) (*ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	catalog, ok := channelCatalogByType(channelType)
	if !ok {
		return nil, ErrChannelNotFound
	}
	if isPlannedChannel(channelType) {
		return nil, invalidChannelControl(errors.New("消息渠道未上线"))
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return nil, invalidChannelControl(errors.New("agent_id is required"))
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}

	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockChannel := s.lockChannelMutation(ownerUserID, channelType)
	defer unlockChannel()

	publicConfig := normalizeStringMap(request.Config)
	secrets := normalizeStringMap(request.Credentials)
	newCredentials := sql.NullString{}
	if len(secrets) > 0 {
		encrypted, encryptErr := s.encryptCredentials(secrets)
		if encryptErr != nil {
			return nil, channelControlMutationFailure(ControlMutationNotApplied, encryptErr)
		}
		newCredentials = sql.NullString{String: encrypted, Valid: true}
	}

	configJSON, err := encodeStringMap(publicConfig)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	reloadSnapshot, err := s.captureChannelReloadSnapshot(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	credentialsEncrypted := newCredentials
	committedVersion, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		existing, loadErr := s.getChannelConfigRowFrom(ctx, tx, ownerUserID, channelType)
		if loadErr != nil {
			return loadErr
		}
		if validateErr := validateChannelConfigInput(
			catalog,
			publicConfig,
			secrets,
			existing != nil && existing.CredentialsEncrypted.Valid,
		); validateErr != nil {
			return validateErr
		}
		if len(secrets) == 0 && existing != nil {
			credentialsEncrypted = existing.CredentialsEncrypted
		}
		return s.upsertChannelConfigRowWith(ctx, tx, channelConfigRow{
			OwnerUserID:          ownerUserID,
			ChannelType:          channelType,
			AgentID:              agentID,
			Status:               ChannelConfigStatusConfigured,
			ConfigJSON:           configJSON,
			CredentialsEncrypted: credentialsEncrypted,
		})
	})
	if err != nil {
		return nil, channelControlVersionError(expectedVersion, err)
	}
	if err = s.reloadChannelRuntime(ctx, ownerUserID, channelType, configJSON, credentialsEncrypted); err != nil {
		if restoreErr := s.restoreChannelReloadSnapshot(
			ctx,
			reloadSnapshot,
			committedVersion,
		); restoreErr != nil {
			return nil, channelControlMutationFailure(ControlMutationUnknown, errors.Join(
				err,
				fmt.Errorf("Channel 候选 runtime 启动失败且恢复上一配置失败: %w", restoreErr),
			))
		}
		return nil, channelControlMutationFailure(
			ControlMutationNotApplied,
			fmt.Errorf("Channel 候选 runtime 启动失败，上一份可运行配置已保留: %w", err),
		)
	}
	view, err := s.channelView(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationCommitted, err)
	}
	return view, nil
}

func (s *ControlService) DeleteChannelConfig(ctx context.Context, ownerUserID string, channelType string) error {
	return s.deleteChannelConfig(ctx, ownerUserID, channelType, 0)
}

func (s *ControlService) DeleteChannelConfigAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedVersion int64,
) error {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return err
	}
	return s.deleteChannelConfig(ctx, ownerUserID, channelType, expectedVersion)
}

func (s *ControlService) deleteChannelConfig(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	expectedVersion int64,
) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockChannel := s.lockChannelMutation(ownerUserID, channelType)
	defer unlockChannel()

	_, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		pairingQuery := "DELETE FROM im_pairings WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2)
		if _, deleteErr := tx.ExecContext(ctx, pairingQuery, ownerUserID, channelType); deleteErr != nil {
			return deleteErr
		}
		if err := s.deleteChannelAccountRowsWith(ctx, tx, ownerUserID, channelType); err != nil {
			return err
		}
		query := "DELETE FROM im_channel_configs WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2)
		_, deleteErr := tx.ExecContext(ctx, query, ownerUserID, channelType)
		return deleteErr
	})
	if err != nil {
		return channelControlVersionError(expectedVersion, err)
	}
	if s.router != nil {
		if err = s.router.UnregisterForOwner(ctx, ownerUserID, channelType); err != nil {
			return channelControlMutationFailure(
				ControlMutationCommitted,
				fmt.Errorf("Channel 配置已删除，但停止 runtime 失败: %w", err),
			)
		}
	}
	return nil
}

func (s *ControlService) DeleteChannelAccount(ctx context.Context, ownerUserID string, channelType string, accountID string) (*ChannelConfigView, error) {
	return s.deleteChannelAccount(ctx, ownerUserID, channelType, accountID, 0)
}

func (s *ControlService) DeleteChannelAccountAtVersion(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
	expectedVersion int64,
) (*ChannelConfigView, error) {
	if _, err := normalizeExpectedChannelControlVersion(expectedVersion); err != nil {
		return nil, err
	}
	return s.deleteChannelAccount(ctx, ownerUserID, channelType, accountID, expectedVersion)
}

func (s *ControlService) deleteChannelAccount(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
	expectedVersion int64,
) (*ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	accountID = strings.TrimSpace(accountID)
	if _, ok := channelCatalogByType(channelType); !ok {
		return nil, ErrChannelNotFound
	}
	if accountID == "" {
		return nil, ErrChannelAccountNotFound
	}
	unlockControl := s.lockControlMutation(ownerUserID)
	defer unlockControl()
	unlockChannel := s.lockChannelMutation(ownerUserID, channelType)
	defer unlockChannel()

	reloadSnapshot, err := s.captureChannelReloadSnapshot(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationNotApplied, err)
	}
	var row *channelConfigRow
	committedVersion, err := s.withChannelControlMutation(ctx, ownerUserID, expectedVersion, func(tx *sql.Tx) error {
		pairingQuery := "DELETE FROM im_pairings WHERE owner_user_id = " + s.bind(1) +
			" AND channel_type = " + s.bind(2) + " AND account_id = " + s.bind(3)
		if _, deleteErr := tx.ExecContext(ctx, pairingQuery, ownerUserID, channelType, accountID); deleteErr != nil {
			return deleteErr
		}
		deleted, deleteErr := s.deleteChannelAccountRowWith(ctx, tx, ownerUserID, channelType, accountID)
		if deleteErr != nil {
			return deleteErr
		}
		if !deleted {
			return ErrChannelAccountNotFound
		}
		row, deleteErr = s.getChannelConfigRowFrom(ctx, tx, ownerUserID, channelType)
		return deleteErr
	})
	if err != nil {
		return nil, channelControlVersionError(expectedVersion, err)
	}
	if row != nil {
		if err = s.reloadChannelRuntime(ctx, ownerUserID, channelType, row.ConfigJSON, row.CredentialsEncrypted); err != nil {
			if restoreErr := s.restoreChannelReloadSnapshot(
				ctx,
				reloadSnapshot,
				committedVersion,
			); restoreErr != nil {
				return nil, channelControlMutationFailure(ControlMutationUnknown, errors.Join(
					err,
					fmt.Errorf("Channel account 热重载失败且恢复上一配置失败: %w", restoreErr),
				))
			}
			return nil, channelControlMutationFailure(
				ControlMutationNotApplied,
				fmt.Errorf("Channel account 热重载失败，上一份可运行配置已保留: %w", err),
			)
		}
	}
	view, err := s.channelView(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, channelControlMutationFailure(ControlMutationCommitted, err)
	}
	return view, nil
}

func (s *ControlService) channelView(ctx context.Context, ownerUserID string, channelType string) (*ChannelConfigView, error) {
	items, err := s.ListChannels(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ChannelType == channelType {
			copyItem := item
			return &copyItem, nil
		}
	}
	return nil, ErrChannelNotFound
}

func (s *ControlService) reloadChannelRuntime(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	configJSON string,
	credentialsEncrypted sql.NullString,
) error {
	runtimeStatus := ChannelConfigStatusConfigured
	runtimeError := ""
	configureErr := s.configureRouterChannel(ctx, ownerUserID, channelType, configJSON, credentialsEncrypted)
	if configureErr != nil {
		runtimeStatus = ChannelConfigStatusError
		runtimeError = configureErr.Error()
	} else if s.router != nil && s.router.IsReadyForOwner(ownerUserID, channelType) {
		runtimeStatus = ChannelConfigStatusConnected
	}
	stateErr := s.updateChannelConfigRuntimeState(ctx, ownerUserID, channelType, runtimeStatus, runtimeError)
	if configureErr != nil {
		if stateErr != nil {
			return errors.Join(configureErr, fmt.Errorf("persist channel runtime error: %w", stateErr))
		}
		return configureErr
	}
	return stateErr
}
