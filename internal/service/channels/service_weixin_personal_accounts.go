package channels

import (
	"context"
	"database/sql"
	"strings"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func (s *ControlService) personalWeixinAccountChannels(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	channelConfig map[string]string,
) ([]*channeladapters.PersonalWeixinChannel, error) {
	rows, err := s.listChannelAccountRows(ctx, ownerUserID, channelType)
	if err != nil {
		return nil, err
	}
	channels := make([]*channeladapters.PersonalWeixinChannel, 0, len(rows))
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled {
			continue
		}
		accountConfig, err := decodeStringMap(row.ConfigJSON)
		if err != nil {
			return nil, err
		}
		secrets, err := s.decryptCredentials(row.CredentialsEncrypted)
		if err != nil {
			return nil, err
		}
		token := strings.TrimSpace(secrets["ilink_bot_token"])
		if token == "" {
			continue
		}
		channels = append(channels, channeladapters.NewPersonalWeixinChannel(channeladapters.PersonalWeixinClientConfig{
			BaseURL:            firstNonEmpty(accountConfig["base_url"], channelConfig["base_url"]),
			Token:              token,
			AccountID:          row.AccountID,
			UserID:             row.UserID,
			BotAgent:           firstNonEmpty(accountConfig["bot_agent"], channelConfig["bot_agent"]),
			IlinkAppID:         firstNonEmpty(accountConfig["ilink_app_id"], channelConfig["ilink_app_id"]),
			IlinkClientVersion: firstNonEmpty(accountConfig["ilink_client_version"], channelConfig["ilink_client_version"]),
		}, s.httpClient).WithOwner(ownerUserID))
	}
	return channels, nil
}

func (s *ControlService) saveLegacyPersonalWeixinAccount(
	ctx context.Context,
	row *channelConfigRow,
	publicConfig map[string]string,
	secrets map[string]string,
) error {
	accountID := strings.TrimSpace(publicConfig["account_id"])
	token := strings.TrimSpace(secrets["ilink_bot_token"])
	if row == nil || accountID == "" || token == "" {
		return nil
	}
	accountConfig, err := personalWeixinAccountConfig(publicConfig, "")
	if err != nil {
		return err
	}
	encrypted, err := s.encryptCredentials(map[string]string{"ilink_bot_token": token})
	if err != nil {
		return err
	}
	return s.upsertChannelAccountRow(ctx, channelAccountRow{
		OwnerUserID: row.OwnerUserID,
		ChannelType: row.ChannelType,
		AccountID:   accountID,
		UserID:      publicConfig["user_id"],
		Status:      ChannelConfigStatusConnected,
		ConfigJSON:  accountConfig,
		CredentialsEncrypted: sql.NullString{
			String: encrypted,
			Valid:  encrypted != "",
		},
	})
}

func (s *ControlService) savePersonalWeixinAccount(
	ctx context.Context,
	row *channelConfigRow,
	publicConfig map[string]string,
	status channeladapters.PersonalWeixinQRStatusResponse,
) error {
	accountID := strings.TrimSpace(status.IlinkBotID)
	token := strings.TrimSpace(status.BotToken)
	if accountID == "" || token == "" {
		return nil
	}
	accountConfig, err := personalWeixinAccountConfig(publicConfig, status.BaseURL)
	if err != nil {
		return err
	}
	encrypted, err := s.encryptCredentials(map[string]string{"ilink_bot_token": token})
	if err != nil {
		return err
	}
	return s.upsertChannelAccountRow(ctx, channelAccountRow{
		OwnerUserID: row.OwnerUserID,
		ChannelType: row.ChannelType,
		AccountID:   accountID,
		UserID:      status.IlinkUserID,
		Status:      ChannelConfigStatusConnected,
		ConfigJSON:  accountConfig,
		CredentialsEncrypted: sql.NullString{
			String: encrypted,
			Valid:  encrypted != "",
		},
	})
}

func (s *ControlService) personalWeixinLocalTokens(
	ctx context.Context,
	row *channelConfigRow,
	secrets map[string]string,
) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0)
	appendToken := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" || seen[token] {
			return
		}
		seen[token] = true
		result = append(result, token)
	}
	appendToken(secrets["ilink_bot_token"])
	if row == nil {
		return result, nil
	}
	rows, err := s.listChannelAccountRows(ctx, row.OwnerUserID, row.ChannelType)
	if err != nil {
		return nil, err
	}
	for _, account := range rows {
		accountSecrets, err := s.decryptCredentials(account.CredentialsEncrypted)
		if err != nil {
			return nil, err
		}
		appendToken(accountSecrets["ilink_bot_token"])
	}
	return result, nil
}

func personalWeixinAccountConfig(publicConfig map[string]string, baseURL string) (string, error) {
	values := map[string]string{
		"base_url":             firstNonEmpty(baseURL, publicConfig["base_url"], channeladapters.DefaultPersonalWeixinBaseURL),
		"bot_agent":            publicConfig["bot_agent"],
		"ilink_app_id":         publicConfig["ilink_app_id"],
		"ilink_client_version": publicConfig["ilink_client_version"],
	}
	return encodeStringMap(values)
}
