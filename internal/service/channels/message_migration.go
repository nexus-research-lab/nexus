package channels

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func migrateIngressMessage(
	request IngressRequest,
	channelStored string,
	parsed protocol.SessionKey,
	content string,
	reqID string,
) *channelmessage.Inbound {
	return channelmessage.NormalizeInbound(request.Message, channelmessage.InboundParams{
		Channel:           channelStored,
		Target:            parsed.Ref,
		PlatformMessageID: reqID,
		ThreadID:          parsed.ThreadID,
		SenderName:        request.ExternalName,
		ChatType:          parsed.ChatType,
		Text:              content,
	})
}
