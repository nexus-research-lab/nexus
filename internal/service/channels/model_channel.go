package channels

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const (
	// DeliveryModeNone 表示不做外部投递。
	DeliveryModeNone = channelcontract.DeliveryModeNone
	// DeliveryModeLast 表示投递到最近一次成功目标。
	DeliveryModeLast = channelcontract.DeliveryModeLast
	// DeliveryModeExplicit 表示投递到显式目标。
	DeliveryModeExplicit = channelcontract.DeliveryModeExplicit

	// ChannelTypeWebSocket 表示浏览器 WebSocket 面板。
	ChannelTypeWebSocket = channelcontract.ChannelTypeWebSocket
	// ChannelTypeDiscord 表示 Discord 通道。
	ChannelTypeDiscord = channelcontract.ChannelTypeDiscord
	// ChannelTypeTelegram 表示 Telegram 通道。
	ChannelTypeTelegram = channelcontract.ChannelTypeTelegram
	// ChannelTypeDingTalk 表示钉钉通道。
	ChannelTypeDingTalk = channelcontract.ChannelTypeDingTalk
	// ChannelTypeWeChat 表示微信通道。
	ChannelTypeWeChat = channelcontract.ChannelTypeWeChat
	// ChannelTypeWeixinPersonal 表示内置个人微信 iLink 通道。
	ChannelTypeWeixinPersonal = protocol.SessionChannelWeixinPersonal
	// ChannelTypeFeishu 表示飞书通道。
	ChannelTypeFeishu = channelcontract.ChannelTypeFeishu
	// ChannelTypeInternal 表示内部系统会话。
	ChannelTypeInternal = channelcontract.ChannelTypeInternal
)

// DeliveryTarget 表示通道无关的投递目标。
type DeliveryTarget = channelcontract.DeliveryTarget

// MessageChannel 定义通道生命周期。
type MessageChannel = channelcontract.MessageChannel

// DeliveryChannel 定义统一文本投递能力。
type DeliveryChannel = channelcontract.DeliveryChannel

// DeliveryResult 表示一次通道投递的目标解析结果与平台回执。
type DeliveryResult = channelcontract.DeliveryResult

func newDeliveryResult(target DeliveryTarget, receipt *channelmessage.Receipt) DeliveryResult {
	return channelcontract.NewDeliveryResult(target, receipt)
}

type agentScopedDeliveryChannel = channelcontract.AgentScopedDeliveryChannel

type typingDeliveryChannel = channelcontract.TypingDeliveryChannel

// IngressRequest 表示一条来自外部通道的标准化消息。
type IngressRequest = channelcontract.IngressRequest

// IngressResult 描述入口受理结果。
type IngressResult = channelcontract.IngressResult

func normalizeChannelType(channel string) string {
	return channelcontract.NormalizeChannelType(channel)
}
