package automation

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type heartbeatFilterResult struct {
	ShouldDeliver bool
	Text          string
}

func toChannelDeliveryTarget(target DeliveryTarget) channels.DeliveryTarget {
	return channels.DeliveryTarget{
		Mode:      strings.TrimSpace(target.Mode),
		Channel:   strings.TrimSpace(target.Channel),
		To:        strings.TrimSpace(target.To),
		AccountID: strings.TrimSpace(target.AccountID),
		ThreadID:  strings.TrimSpace(target.ThreadID),
	}.Normalized()
}

func filterHeartbeatResponse(text string, ackMaxChars int) heartbeatFilterResult {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return heartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}
	if normalized == "HEARTBEAT_OK" {
		return heartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}

	stripped := normalized
	prefixRemoved := false
	suffixRemoved := false
	if strings.HasPrefix(stripped, "HEARTBEAT_OK") {
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "HEARTBEAT_OK"))
		prefixRemoved = true
	}
	if strings.HasSuffix(stripped, "HEARTBEAT_OK") {
		stripped = strings.TrimSpace(strings.TrimSuffix(stripped, "HEARTBEAT_OK"))
		suffixRemoved = true
	}
	if !prefixRemoved && !suffixRemoved {
		return heartbeatFilterResult{ShouldDeliver: true, Text: text}
	}
	if len([]rune(stripped)) <= ackMaxChars {
		return heartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}
	return heartbeatFilterResult{ShouldDeliver: true, Text: stripped}
}
