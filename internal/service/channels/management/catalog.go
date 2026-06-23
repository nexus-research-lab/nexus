package management

import (
	"slices"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

const (
	weComBotDefaultLongConnectionURL   = "wss://openws.work.weixin.qq.com"
	defaultPersonalWeixinBaseURL       = "https://ilinkai.weixin.qq.com"
	defaultPersonalWeixinAppID         = "bot"
	defaultPersonalWeixinClientVersion = "132099"
	defaultPersonalWeixinBotAgent      = "Nexus/0.1.0"
)

func ChannelCatalog() []ChannelCatalogItem {
	items := []ChannelCatalogItem{
		{
			ChannelType:   channelcontract.ChannelTypeDingTalk,
			Title:         "钉钉",
			BotLabel:      "钉钉机器人",
			Description:   "通过钉钉应用机器人接收群聊或单聊消息，并把任务结果回投到钉钉会话。",
			DocsURL:       "https://opensource.dingtalk.com/developerpedia/docs/learn/bot/appbot/receive/",
			RuntimeStatus: "ready",
			RuntimeNote:   "使用官方 Stream 模式接收入站消息；收到消息后的回复优先使用 sessionWebhook，Robot Code 仅用于显式 openConversationId 主动群发。",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "client_id", Label: "Client ID（AppKey）", Kind: "text", Required: true, Placeholder: "填写开发者控制台的 Client ID"},
				{Key: "client_secret", Label: "Client Secret（AppSecret）", Kind: "password", Required: true, Secret: true, Placeholder: "填写开发者控制台的 Client Secret"},
				{Key: "robot_code", Label: "Robot Code", Kind: "text", Placeholder: "可选；仅用于主动群发 OpenAPI"},
				{Key: "base_url", Label: "OpenAPI Base URL", Kind: "text", Placeholder: "https://api.dingtalk.com"},
				{Key: "stream_base_url", Label: "Stream Base URL", Kind: "text", Placeholder: "https://api.dingtalk.com"},
			},
		},
		{
			ChannelType:   channelcontract.ChannelTypeWeChat,
			Title:         "企业微信",
			BotLabel:      "企业微信智能机器人",
			Description:   "通过企业微信智能机器人长连接接收成员或群消息，并使用原生 stream 回复到会话。",
			DocsURL:       "https://developer.work.weixin.qq.com/",
			RuntimeStatus: "ready",
			RuntimeNote:   "使用企业微信智能机器人长连接；只需要 Bot ID 和 Secret，Nexus 会接收入站消息并用 stream 回复。",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "bot_id", Label: "Bot ID", Kind: "text", Required: true, Placeholder: "填写企业微信智能机器人 Bot ID"},
				{Key: "secret", Label: "Secret", Kind: "password", Required: true, Secret: true, Placeholder: "填写企业微信智能机器人 Secret"},
				{Key: "base_url", Label: "Long Connection URL", Kind: "text", Placeholder: weComBotDefaultLongConnectionURL},
			},
		},
		{
			ChannelType:    channelcontract.ChannelTypeWeixinPersonal,
			Title:          "微信",
			BotLabel:       "微信 iLink Bot",
			Description:    "通过腾讯 iLink Bot API 接入微信私聊，Nexus 内置扫码登录、消息长轮询和文本回投。",
			RuntimeStatus:  "ready",
			RuntimeNote:    "使用腾讯 iLink Bot API；扫码登录后 Nexus 保存 ilink_bot_token 并直接长轮询 getUpdates、调用 sendMessage 回投文本。",
			SupportsGroup:  false,
			SupportsQRCode: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "base_url", Label: "iLink API Base URL", Kind: "text", Placeholder: defaultPersonalWeixinBaseURL},
				{Key: "bot_agent", Label: "Bot Agent", Kind: "text", Placeholder: defaultPersonalWeixinBotAgent},
				{Key: "ilink_app_id", Label: "iLink App ID", Kind: "text", Placeholder: defaultPersonalWeixinAppID},
				{Key: "ilink_client_version", Label: "iLink Client Version", Kind: "text", Placeholder: defaultPersonalWeixinClientVersion},
			},
		},
		{
			ChannelType:   channelcontract.ChannelTypeFeishu,
			Title:         "飞书",
			BotLabel:      "飞书机器人",
			Description:   "通过飞书自建应用机器人收发群聊或单聊消息，默认使用长连接事件订阅，不需要公网回调地址。",
			DocsURL:       "https://open.feishu.cn/document/server-docs/event-subscription-guide/event-subscription-configure-/request-url-configuration-case",
			RuntimeStatus: "ready",
			RuntimeNote:   "默认使用飞书长连接事件订阅接收入站消息；支持消息 reply、typing reaction 和 reaction.created 通知。Webhook 回调仍作为兼容模式保留。",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "app_id", Label: "App ID", Kind: "text", Required: true, Placeholder: "例如 cli_xxxxxxxxx"},
				{Key: "app_secret", Label: "App Secret", Kind: "password", Required: true, Secret: true, Placeholder: "填写应用 App Secret"},
				{Key: "connection_mode", Label: "Connection Mode", Kind: "text", Placeholder: "websocket 或 webhook，默认 websocket"},
				{Key: "base_url", Label: "OpenAPI Base URL", Kind: "text", Placeholder: "https://open.feishu.cn"},
				{Key: "reply_in_thread", Label: "Reply In Thread", Kind: "text", Placeholder: "true/false，默认 false"},
				{Key: "verification_token", Label: "Verification Token", Kind: "password", Secret: true, Placeholder: "可选：填写事件订阅 Verification Token"},
				{Key: "encrypt_key", Label: "Encrypt Key", Kind: "password", Secret: true, Placeholder: "可选：填写事件订阅 Encrypt Key"},
			},
		},
		{
			ChannelType:   channelcontract.ChannelTypeTelegram,
			Title:         "Telegram",
			BotLabel:      "Telegram Bot",
			Description:   "通过 Telegram Bot API 接收私聊/群聊消息，并向聊天或话题回投文本。",
			DocsURL:       "https://core.telegram.org/bots",
			RuntimeStatus: "ready",
			RuntimeNote:   "使用 Bot API getUpdates 长轮询；支持 edited_message、话题 message_thread_id、sendMessage 和 sendChatAction typing。",
			SupportsGroup: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "bot_token", Label: "Bot Token", Kind: "password", Required: true, Secret: true, Placeholder: "粘贴来自 @BotFather 的 Token"},
				{Key: "base_url", Label: "Bot API Base URL", Kind: "text", Placeholder: "https://api.telegram.org"},
			},
		},
		{
			ChannelType:       channelcontract.ChannelTypeDiscord,
			Title:             "Discord",
			BotLabel:          "Discord Bot",
			Description:       "通过 Discord Bot 接收服务器频道、Thread 或私聊消息，并向频道回投文本。",
			DocsURL:           "https://discord.com/developers/docs/resources/message#create-message",
			RuntimeStatus:     "ready",
			RuntimeNote:       "使用 Discord Bot Token 连接 Gateway 接收消息，并调用 REST create message/typing 回投；Application ID 仅用于生成 Bot 授权链接。",
			SupportsGroup:     true,
			SupportsOAuthLink: true,
			CredentialFields: []ChannelCredentialField{
				{Key: "application_id", Label: "Application ID（Client ID）", Kind: "text", Required: true, Placeholder: "填写 General Information / OAuth2 中的 Application ID"},
				{Key: "bot_token", Label: "Bot Token（Reset Token）", Kind: "password", Required: true, Secret: true, Placeholder: "填写 Bot 页面生成的 Token，不是 Client Secret"},
				{Key: "base_url", Label: "REST API Base URL", Kind: "text", Placeholder: "https://discord.com/api/v10"},
			},
		},
	}
	for index := range items {
		items[index].Capabilities = ChannelCapabilities(items[index].ChannelType)
	}
	return items
}

func ChannelCatalogByType(channelType string) (ChannelCatalogItem, bool) {
	channelType = channelcontract.NormalizeChannelType(channelType)
	items := ChannelCatalog()
	index := slices.IndexFunc(items, func(item ChannelCatalogItem) bool {
		return item.ChannelType == channelType
	})
	if index < 0 {
		return ChannelCatalogItem{}, false
	}
	return items[index], true
}

func IsPlannedChannel(channelType string) bool {
	item, ok := ChannelCatalogByType(channelType)
	return ok && item.RuntimeStatus == "planned"
}
