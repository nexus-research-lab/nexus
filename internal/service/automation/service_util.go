package automation

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

func errorPointer(err error) *string {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	return &message
}

func anyStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func deliveryTargetSummary(target protocol.DeliveryTarget) string {
	mode := strings.TrimSpace(target.Mode)
	switch mode {
	case "", protocol.DeliveryModeNone:
		return ""
	case protocol.DeliveryModeLast:
		return protocol.DeliveryModeLast
	case protocol.DeliveryModeExplicit:
		parts := []string{protocol.DeliveryModeExplicit}
		if channel := strings.TrimSpace(target.Channel); channel != "" {
			parts = append(parts, channel)
		}
		if to := strings.TrimSpace(target.To); to != "" {
			parts = append(parts, to)
		}
		if threadID := strings.TrimSpace(target.ThreadID); threadID != "" {
			parts = append(parts, "thread:"+threadID)
		}
		return strings.Join(parts, ":")
	default:
		return mode
	}
}
