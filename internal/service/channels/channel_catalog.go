package channels

import channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"

func channelCatalog() []ChannelCatalogItem {
	return channelmanagement.ChannelCatalog()
}

func channelCatalogByType(channelType string) (ChannelCatalogItem, bool) {
	return channelmanagement.ChannelCatalogByType(channelType)
}

func isPlannedChannel(channelType string) bool {
	return channelmanagement.IsPlannedChannel(channelType)
}

func sortedChannelTypes() []string {
	return channelmanagement.SortedChannelTypes()
}
