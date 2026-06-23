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
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	catalog, ok := channelCatalogByType(channelType)
	if !ok {
		return nil, ErrChannelNotFound
	}
	if isPlannedChannel(channelType) {
		return nil, errors.New("消息渠道未上线")
	}
	agentID := strings.TrimSpace(request.AgentID)
	if agentID == "" {
		return nil, errors.New("agent_id is required")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}

	publicConfig := normalizeStringMap(request.Config)
	secrets := normalizeStringMap(request.Credentials)
	existing, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, err
	}
	if err = validateChannelConfigInput(catalog, publicConfig, secrets, existing != nil && existing.CredentialsEncrypted.Valid); err != nil {
		return nil, err
	}
	credentialsEncrypted := sql.NullString{}
	if len(secrets) > 0 {
		encrypted, encryptErr := s.encryptCredentials(secrets)
		if encryptErr != nil {
			return nil, encryptErr
		}
		credentialsEncrypted = sql.NullString{String: encrypted, Valid: true}
	} else if existing != nil {
		credentialsEncrypted = existing.CredentialsEncrypted
	}

	configJSON, err := encodeStringMap(publicConfig)
	if err != nil {
		return nil, err
	}
	if err = s.upsertChannelConfigRow(ctx, channelConfigRow{
		OwnerUserID:          ownerUserID,
		ChannelType:          channelType,
		AgentID:              agentID,
		Status:               ChannelConfigStatusConfigured,
		ConfigJSON:           configJSON,
		CredentialsEncrypted: credentialsEncrypted,
	}); err != nil {
		return nil, err
	}
	if err = s.reloadChannelRuntime(ctx, ownerUserID, channelType, configJSON, credentialsEncrypted); err != nil {
		return nil, err
	}
	return s.channelView(ctx, ownerUserID, channelType)
}

func (s *ControlService) DeleteChannelConfig(ctx context.Context, ownerUserID string, channelType string) error {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	query := "DELETE FROM im_channel_configs WHERE owner_user_id = " + s.bind(1) + " AND channel_type = " + s.bind(2)
	_, err := s.db.ExecContext(ctx, query, ownerUserID, channelType)
	if err == nil {
		err = s.deleteChannelAccountRows(ctx, ownerUserID, channelType)
	}
	if err == nil && s.router != nil {
		s.router.UnregisterForOwner(ctx, ownerUserID, channelType)
	}
	return err
}

func (s *ControlService) DeleteChannelAccount(ctx context.Context, ownerUserID string, channelType string, accountID string) (*ChannelConfigView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	accountID = strings.TrimSpace(accountID)
	if _, ok := channelCatalogByType(channelType); !ok {
		return nil, ErrChannelNotFound
	}
	if accountID == "" {
		return nil, ErrChannelAccountNotFound
	}
	deleted, err := s.deleteChannelAccountRow(ctx, ownerUserID, channelType, accountID)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, ErrChannelAccountNotFound
	}
	row, err := s.getChannelConfigRow(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, err
	}
	if row != nil {
		if err = s.reloadChannelRuntime(ctx, ownerUserID, channelType, row.ConfigJSON, row.CredentialsEncrypted); err != nil {
			return nil, err
		}
	}
	return s.channelView(ctx, ownerUserID, channelType)
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
	if err := s.configureRouterChannel(ctx, ownerUserID, channelType, configJSON, credentialsEncrypted); err != nil {
		runtimeStatus = ChannelConfigStatusError
		runtimeError = err.Error()
	} else if s.router != nil && s.router.IsReadyForOwner(ownerUserID, channelType) {
		runtimeStatus = ChannelConfigStatusConnected
	}
	return s.updateChannelConfigRuntimeState(ctx, ownerUserID, channelType, runtimeStatus, runtimeError)
}
