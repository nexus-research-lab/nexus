package channels

import (
	"context"
	"database/sql"
	"strings"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

func (s *ControlService) LoadConfiguredChannels(ctx context.Context) error {
	rows, err := s.listAllChannelConfigRows(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == ChannelConfigStatusDisabled {
			continue
		}
		if err := s.configureRouterChannel(ctx, row.OwnerUserID, row.ChannelType, row.ConfigJSON, row.CredentialsEncrypted); err != nil {
			_ = s.updateChannelConfigRuntimeState(ctx, row.OwnerUserID, row.ChannelType, ChannelConfigStatusError, err.Error())
		}
	}
	return nil
}

func (s *ControlService) configureRouterChannel(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	configJSON string,
	encrypted sql.NullString,
) error {
	if s.router == nil {
		return nil
	}
	if isPlannedChannel(channelType) {
		return nil
	}
	secrets, err := s.decryptCredentials(encrypted)
	if err != nil {
		return err
	}
	switch normalizeIMChannelType(channelType) {
	case ChannelTypeFeishu:
		publicConfig, _ := decodeStringMap(configJSON)
		appID := strings.TrimSpace(publicConfig["app_id"])
		appSecret := strings.TrimSpace(secrets["app_secret"])
		if appID == "" || appSecret == "" {
			return nil
		}
		channel := channeladapters.NewFeishuChannel(appID, appSecret, s.httpClient).
			WithOwner(ownerUserID).
			WithEventSecurity(secrets["verification_token"], secrets["encrypt_key"]).
			WithConnectionMode(publicConfig["connection_mode"]).
			WithReplyInThread(publicConfig["reply_in_thread"])
		if baseURL := strings.TrimSpace(publicConfig["base_url"]); baseURL != "" {
			channel.WithBaseURL(baseURL)
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	case ChannelTypeTelegram:
		publicConfig, _ := decodeStringMap(configJSON)
		token := strings.TrimSpace(secrets["bot_token"])
		if token == "" {
			return nil
		}
		channel := channeladapters.NewTelegramChannel(token, s.httpClient).WithOwner(ownerUserID)
		if baseURL := strings.TrimSpace(publicConfig["base_url"]); baseURL != "" {
			channel.WithBaseURL(baseURL)
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	case ChannelTypeDiscord:
		publicConfig, _ := decodeStringMap(configJSON)
		token := strings.TrimSpace(secrets["bot_token"])
		if token == "" {
			return nil
		}
		channel := channeladapters.NewDiscordChannel(token, s.httpClient).WithOwner(ownerUserID)
		if baseURL := strings.TrimSpace(publicConfig["base_url"]); baseURL != "" {
			channel.WithBaseURL(baseURL)
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	case ChannelTypeDingTalk:
		publicConfig, _ := decodeStringMap(configJSON)
		clientID := strings.TrimSpace(publicConfig["client_id"])
		clientSecret := strings.TrimSpace(secrets["client_secret"])
		robotCode := strings.TrimSpace(publicConfig["robot_code"])
		if clientID == "" || clientSecret == "" {
			return nil
		}
		channel := channeladapters.NewDingTalkChannel(clientID, clientSecret, robotCode, s.httpClient).WithOwner(ownerUserID)
		if baseURL := strings.TrimSpace(publicConfig["base_url"]); baseURL != "" {
			channel.WithBaseURL(baseURL)
		}
		if streamBaseURL := strings.TrimSpace(publicConfig["stream_base_url"]); streamBaseURL != "" {
			channel.WithStreamHost(streamBaseURL)
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	case ChannelTypeWeChat:
		publicConfig, _ := decodeStringMap(configJSON)
		botID := strings.TrimSpace(publicConfig["bot_id"])
		secret := strings.TrimSpace(secrets["secret"])
		if botID == "" || secret == "" {
			return nil
		}
		channel := channeladapters.NewWeComBotChannel(botID, secret).WithOwner(ownerUserID)
		if baseURL := strings.TrimSpace(publicConfig["base_url"]); baseURL != "" {
			channel.WithBaseURL(baseURL)
		}
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	case ChannelTypeWeixinPersonal:
		publicConfig, _ := decodeStringMap(configJSON)
		channels, err := s.personalWeixinAccountChannels(ctx, ownerUserID, channelType, publicConfig)
		if err != nil {
			return err
		}
		if len(channels) > 1 {
			return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channeladapters.NewPersonalWeixinMultiAccountChannel(channels))
		}
		if len(channels) == 1 {
			return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channels[0])
		}
		token := strings.TrimSpace(secrets["ilink_bot_token"])
		if token == "" {
			s.router.UnregisterForOwner(ctx, ownerUserID, channelType)
			return nil
		}
		channel := channeladapters.NewPersonalWeixinChannel(channeladapters.PersonalWeixinClientConfig{
			BaseURL:            publicConfig["base_url"],
			Token:              token,
			AccountID:          publicConfig["account_id"],
			UserID:             publicConfig["user_id"],
			BotAgent:           publicConfig["bot_agent"],
			IlinkAppID:         publicConfig["ilink_app_id"],
			IlinkClientVersion: publicConfig["ilink_client_version"],
		}, s.httpClient).WithOwner(ownerUserID)
		return s.router.RegisterAndStartForOwner(ctx, ownerUserID, channel)
	default:
		return nil
	}
}

func (s *ControlService) connectionStateFor(ownerUserID string, channelType string, status string) string {
	status = firstNonEmpty(status, ChannelConfigStatusConfigured)
	if status == ChannelConfigStatusDisabled {
		return "disabled"
	}
	if status == ChannelConfigStatusError {
		return "error"
	}
	if s.router != nil && s.router.IsReadyForOwner(ownerUserID, channelType) {
		return "connected"
	}
	if status == ChannelConfigStatusConnected {
		return ChannelConfigStatusConfigured
	}
	return status
}
