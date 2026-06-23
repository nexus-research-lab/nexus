package channels

import (
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

func dmExternalReplyTarget(target *DeliveryTarget) *dmsvc.ExternalReplyTarget {
	if target == nil {
		return nil
	}
	normalized := target.Normalized()
	if normalized.Mode == DeliveryModeNone {
		return nil
	}
	switch normalized.Channel {
	case "", ChannelTypeWebSocket, ChannelTypeInternal:
		return nil
	}
	return &dmsvc.ExternalReplyTarget{
		Mode:       normalized.Mode,
		Channel:    normalized.Channel,
		To:         normalized.To,
		AccountID:  normalized.AccountID,
		ThreadID:   normalized.ThreadID,
		SessionKey: normalized.SessionKey,
	}
}
