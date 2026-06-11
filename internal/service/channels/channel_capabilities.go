package channels

import channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"

func channelCapabilityMatrix(channelType string) channelmessage.CapabilitySet {
	switch normalizeChannelType(channelType) {
	case ChannelTypeDiscord:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case ChannelTypeTelegram:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case ChannelTypeFeishu:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityThread,
			channelmessage.CapabilityReply,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case ChannelTypeWeixinPersonal:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityTyping,
			channelmessage.CapabilityReply,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	case ChannelTypeDingTalk, ChannelTypeWeChat:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityDurableFinal,
		)
	case ChannelTypeInternal, ChannelTypeWebSocket:
		return channelmessage.NewCapabilitySet(
			channelmessage.CapabilityText,
			channelmessage.CapabilityReceipt,
			channelmessage.CapabilityDurableFinal,
		)
	default:
		return channelmessage.NewCapabilitySet()
	}
}

func channelCapabilities(channelType string) []channelmessage.Capability {
	set := channelCapabilityMatrix(channelType)
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
