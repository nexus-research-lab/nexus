package automation

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func toChannelDeliveryTarget(target protocol.DeliveryTarget) channels.DeliveryTarget {
	return channels.DeliveryTarget{
		Mode:      strings.TrimSpace(target.Mode),
		Channel:   strings.TrimSpace(target.Channel),
		To:        strings.TrimSpace(target.To),
		AccountID: strings.TrimSpace(target.AccountID),
		ThreadID:  strings.TrimSpace(target.ThreadID),
	}.Normalized()
}
