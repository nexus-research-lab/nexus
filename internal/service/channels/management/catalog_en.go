// INPUT: 频道目录视图与请求的展示语言。
// OUTPUT: 英文界面使用的频道标题、说明、运行提示和凭据字段文案。
// POS: channels/management 目录文案的英文投影；中文目录仍是真相源，不改变协议值或配置键。
package management

import (
	"slices"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

type channelCatalogText struct {
	title       string
	botLabel    string
	description string
	runtimeNote string
	fields      map[string]channelFieldText
}

type channelFieldText struct {
	label       string
	placeholder string
}

var englishChannelCatalog = map[string]channelCatalogText{
	channelcontract.ChannelTypeDingTalk: {
		title:       "DingTalk",
		botLabel:    "DingTalk bot",
		description: "Receive group and direct messages through a DingTalk app bot and send task results back to the conversation.",
		runtimeNote: "Scan with DingTalk to create a bot in one step and receive messages over the official Stream mode. You can also enter app credentials manually.",
		fields: map[string]channelFieldText{
			"client_id":       {label: "Client ID (optional for manual setup)", placeholder: "Filled in when you scan, or enter the AppKey"},
			"client_secret":   {label: "Client Secret (optional for manual setup)", placeholder: "Saved when you scan, or enter the AppSecret"},
			"robot_code":      {label: "Robot Code", placeholder: "Optional. Only for proactive group messages over OpenAPI"},
			"base_url":        {label: "OpenAPI Base URL", placeholder: "https://api.dingtalk.com"},
			"stream_base_url": {label: "Stream Base URL", placeholder: "https://api.dingtalk.com"},
		},
	},
	channelcontract.ChannelTypeWeChat: {
		title:       "WeCom",
		botLabel:    "WeCom intelligent bot",
		description: "Receive member and group messages over a WeCom intelligent bot persistent connection and reply in the conversation with native streaming.",
		runtimeNote: "Scan with WeCom to bind an intelligent bot and open a persistent connection. You can also enter the Bot ID and Secret manually.",
		fields: map[string]channelFieldText{
			"bot_id":   {label: "Bot ID (optional for manual setup)", placeholder: "Filled in when you scan, or enter the Bot ID"},
			"secret":   {label: "Secret (optional for manual setup)", placeholder: "Saved when you scan, or enter the Secret"},
			"base_url": {label: "Long Connection URL", placeholder: weComBotDefaultLongConnectionURL},
		},
	},
	channelcontract.ChannelTypeWeixinPersonal: {
		title:       "WeChat",
		botLabel:    "WeChat iLink Bot",
		description: "Connect WeChat direct messages through Tencent's iLink Bot API, with built-in QR sign-in, long polling and text replies.",
		runtimeNote: "Uses Tencent's iLink Bot API. After QR sign-in, Nexus saves the ilink_bot_token, long-polls getUpdates and replies with sendMessage.",
		fields: map[string]channelFieldText{
			"base_url":             {label: "iLink API Base URL", placeholder: defaultPersonalWeixinBaseURL},
			"bot_agent":            {label: "Bot Agent", placeholder: defaultPersonalWeixinBotAgent},
			"ilink_app_id":         {label: "iLink App ID", placeholder: defaultPersonalWeixinAppID},
			"ilink_client_version": {label: "iLink Client Version", placeholder: defaultPersonalWeixinClientVersion},
		},
	},
	channelcontract.ChannelTypeFeishu: {
		title:       "Feishu",
		botLabel:    "Feishu bot",
		description: "Send and receive group and direct messages through a Feishu custom app bot. It uses persistent event subscriptions by default, so no public callback URL is needed.",
		runtimeNote: "On the Feishu QR page, pick an existing app or create and authorize a new one. You can also enter an existing App ID and Secret manually. Messages arrive over a persistent connection by default.",
		fields: map[string]channelFieldText{
			"app_id":             {label: "App ID (optional for manual setup)", placeholder: "Filled in when you scan, or enter cli_xxxxxxxxx"},
			"app_secret":         {label: "App Secret (optional for manual setup)", placeholder: "Saved when you scan, or enter the App Secret"},
			"connection_mode":    {label: "Event delivery mode", placeholder: "websocket or webhook. Defaults to websocket"},
			"base_url":           {label: "OpenAPI Base URL", placeholder: "https://open.feishu.cn"},
			"reply_in_thread":    {label: "Reply in threads", placeholder: "true or false. Defaults to false. When on, each new thread must be paired again in Nexus"},
			"verification_token": {label: "Encryption: Verification Token", placeholder: "Optional. Event subscription Verification Token"},
			"encrypt_key":        {label: "Encryption: Encrypt Key", placeholder: "Optional. Event subscription Encrypt Key"},
		},
	},
	channelcontract.ChannelTypeTelegram: {
		title:       "Telegram",
		botLabel:    "Telegram Bot",
		description: "Receive direct and group messages through the Telegram Bot API and reply with text in chats or topics.",
		runtimeNote: "Uses Bot API getUpdates long polling. Supports edited_message, topic message_thread_id, sendMessage and the sendChatAction typing indicator.",
		fields: map[string]channelFieldText{
			"bot_token": {label: "Bot Token", placeholder: "Paste the token from @BotFather"},
			"base_url":  {label: "Bot API Base URL", placeholder: "https://api.telegram.org"},
		},
	},
	channelcontract.ChannelTypeDiscord: {
		title:       "Discord",
		botLabel:    "Discord Bot",
		description: "Receive messages from server channels, threads and direct messages through a Discord bot and reply with text in the channel.",
		runtimeNote: "Connects to the Gateway with the Discord Bot Token to receive messages and replies through the REST create message and typing endpoints. The Application ID is only used to build the bot authorization link.",
		fields: map[string]channelFieldText{
			"application_id": {label: "Application ID (Client ID)", placeholder: "The Application ID under General Information / OAuth2"},
			"bot_token":      {label: "Bot Token (Reset Token)", placeholder: "The token generated on the Bot page, not the Client Secret"},
			"base_url":       {label: "REST API Base URL", placeholder: "https://discord.com/api/v10"},
		},
	},
}

// LocalizeChannelConfigView 把频道视图中的目录文案投影为请求语言；
// 仅英文界面替换文案，其余语言保留中文目录原文。
func LocalizeChannelConfigView(view ChannelConfigView, locale string) ChannelConfigView {
	if !isEnglishLocale(locale) {
		return view
	}
	text, ok := englishChannelCatalog[view.ChannelType]
	if !ok {
		return view
	}
	view.Title = text.title
	view.BotLabel = text.botLabel
	view.Description = text.description
	view.RuntimeNote = text.runtimeNote
	view.CredentialFields = slices.Clone(view.CredentialFields)
	for index, field := range view.CredentialFields {
		if fieldText, ok := text.fields[field.Key]; ok {
			view.CredentialFields[index].Label = fieldText.label
			view.CredentialFields[index].Placeholder = fieldText.placeholder
		}
	}
	return view
}

// isEnglishLocale 只接受 en 及其地区变体（en-US、en_GB）；其余值一律回退中文目录。
func isEnglishLocale(locale string) bool {
	language, _, _ := strings.Cut(strings.ReplaceAll(strings.ToLower(locale), "_", "-"), "-")
	return language == "en"
}
