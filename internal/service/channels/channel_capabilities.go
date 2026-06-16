package channels

import (
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func channelCapabilityMatrix(channelType string) channelmessage.CapabilitySet {
	return channelmanagement.ChannelCapabilityMatrix(channelType)
}

func channelCapabilities(channelType string) []channelmessage.Capability {
	return channelmanagement.ChannelCapabilities(channelType)
}
