package management

import (
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func ChannelCapabilityMatrix(channelType string) channelmessage.CapabilitySet {
	switch channelcontract.NormalizeChannelType(channelType) {
	case channelcontract.ChannelTypeDiscord:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case channelcontract.ChannelTypeTelegram:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case channelcontract.ChannelTypeFeishu:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReply,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case channelcontract.ChannelTypeWeixinPersonal:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityReply,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case channelcontract.ChannelTypeDingTalk, channelcontract.ChannelTypeWeChat:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityDurableFinal,
		)
	case channelcontract.ChannelTypeInternal, channelcontract.ChannelTypeWebSocket:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	default:
		return channelmessage.NewCapabilitySet()
	}
}

func ChannelCapabilities(channelType string) []channelmessage.Capability {
	set := ChannelCapabilityMatrix(channelType)
	ordered := []channelmessage.Capability{
		channelmessage.CapabilityText,
		channelmessage.CapabilityMedia,
		channelmessage.CapabilityTyping,
		channelmessage.CapabilityThread,
		channelmessage.CapabilityReply,
		channelmessage.CapabilityReceipt,
		channelmessage.CapabilityDurableFinal,
	}
	result := make([]channelmessage.Capability, 0, len(ordered))
	for _, capability := range ordered {
		if set.Has(capability) {
			result = append(result, capability)
		}
	}
	return result
}
