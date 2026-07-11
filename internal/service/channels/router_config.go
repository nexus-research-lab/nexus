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
	configurer := routerChannelConfigurers[normalizeIMChannelType(channelType)]
	if configurer == nil {
		return nil
	}
	secrets, err := s.decryptCredentials(encrypted)
	if err != nil {
		return err
	}
	publicConfig, _ := decodeStringMap(configJSON)
	return configurer(s, ctx, routerChannelConfiguration{
		ownerUserID: ownerUserID,
		channelType: channelType,
		public:      publicConfig,
		secrets:     secrets,
	})
}

type routerChannelConfiguration struct {
	ownerUserID string
	channelType string
	public      map[string]string
	secrets     map[string]string
}

type routerChannelConfigurer func(*ControlService, context.Context, routerChannelConfiguration) error

var routerChannelConfigurers = map[string]routerChannelConfigurer{
	ChannelTypeFeishu:         (*ControlService).configureFeishuRouterChannel,
	ChannelTypeTelegram:       (*ControlService).configureTelegramRouterChannel,
	ChannelTypeDiscord:        (*ControlService).configureDiscordRouterChannel,
	ChannelTypeDingTalk:       (*ControlService).configureDingTalkRouterChannel,
	ChannelTypeWeChat:         (*ControlService).configureWeChatRouterChannel,
	ChannelTypeWeixinPersonal: (*ControlService).configurePersonalWeixinRouterChannel,
}

func (s *ControlService) configureFeishuRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	appID := strings.TrimSpace(config.public["app_id"])
	appSecret := strings.TrimSpace(config.secrets["app_secret"])
	if appID == "" || appSecret == "" {
		return nil
	}
	channel := channeladapters.NewFeishuChannel(appID, appSecret, s.httpClient).
		WithOwner(config.ownerUserID).
		WithEventSecurity(config.secrets["verification_token"], config.secrets["encrypt_key"]).
		WithConnectionMode(config.public["connection_mode"]).
		WithReplyInThread(config.public["reply_in_thread"])
	if baseURL := strings.TrimSpace(config.public["base_url"]); baseURL != "" {
		channel.WithBaseURL(baseURL)
	}
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) configureTelegramRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	token := strings.TrimSpace(config.secrets["bot_token"])
	if token == "" {
		return nil
	}
	channel := channeladapters.NewTelegramChannel(token, s.httpClient).WithOwner(config.ownerUserID)
	if baseURL := strings.TrimSpace(config.public["base_url"]); baseURL != "" {
		channel.WithBaseURL(baseURL)
	}
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) configureDiscordRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	token := strings.TrimSpace(config.secrets["bot_token"])
	if token == "" {
		return nil
	}
	channel := channeladapters.NewDiscordChannel(token, s.httpClient).WithOwner(config.ownerUserID)
	if baseURL := strings.TrimSpace(config.public["base_url"]); baseURL != "" {
		channel.WithBaseURL(baseURL)
	}
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) configureDingTalkRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	clientID := strings.TrimSpace(config.public["client_id"])
	clientSecret := strings.TrimSpace(config.secrets["client_secret"])
	if clientID == "" || clientSecret == "" {
		return nil
	}
	channel := channeladapters.NewDingTalkChannel(
		clientID,
		clientSecret,
		strings.TrimSpace(config.public["robot_code"]),
		s.httpClient,
	).WithOwner(config.ownerUserID)
	if baseURL := strings.TrimSpace(config.public["base_url"]); baseURL != "" {
		channel.WithBaseURL(baseURL)
	}
	if streamBaseURL := strings.TrimSpace(config.public["stream_base_url"]); streamBaseURL != "" {
		channel.WithStreamHost(streamBaseURL)
	}
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) configureWeChatRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	botID := strings.TrimSpace(config.public["bot_id"])
	secret := strings.TrimSpace(config.secrets["secret"])
	if botID == "" || secret == "" {
		return nil
	}
	channel := channeladapters.NewWeComBotChannel(botID, secret).WithOwner(config.ownerUserID)
	if baseURL := strings.TrimSpace(config.public["base_url"]); baseURL != "" {
		channel.WithBaseURL(baseURL)
	}
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) configurePersonalWeixinRouterChannel(ctx context.Context, config routerChannelConfiguration) error {
	channels, err := s.personalWeixinAccountChannels(ctx, config.ownerUserID, config.channelType, config.public)
	if err != nil {
		return err
	}
	if len(channels) > 1 {
		return s.registerConfiguredChannel(ctx, config, channeladapters.NewPersonalWeixinMultiAccountChannel(channels))
	}
	if len(channels) == 1 {
		return s.registerConfiguredChannel(ctx, config, channels[0])
	}
	token := strings.TrimSpace(config.secrets["ilink_bot_token"])
	if token == "" {
		s.router.UnregisterForOwner(ctx, config.ownerUserID, config.channelType)
		return nil
	}
	channel := channeladapters.NewPersonalWeixinChannel(channeladapters.PersonalWeixinClientConfig{
		BaseURL:            config.public["base_url"],
		Token:              token,
		AccountID:          config.public["account_id"],
		UserID:             config.public["user_id"],
		BotAgent:           config.public["bot_agent"],
		IlinkAppID:         config.public["ilink_app_id"],
		IlinkClientVersion: config.public["ilink_client_version"],
	}, s.httpClient).WithOwner(config.ownerUserID)
	return s.registerConfiguredChannel(ctx, config, channel)
}

func (s *ControlService) registerConfiguredChannel(
	ctx context.Context,
	config routerChannelConfiguration,
	channel DeliveryChannel,
) error {
	return s.router.RegisterAndStartForOwner(ctx, config.ownerUserID, channel)
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
