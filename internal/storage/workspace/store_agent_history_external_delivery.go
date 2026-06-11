package workspace

import (
	"fmt"
	"strings"
	"time"
)

func normalizeExternalDeliveryReceipt(receipt ExternalDeliveryReceipt) ExternalDeliveryReceipt {
	normalized := ExternalDeliveryReceipt{
		RoundID:                  strings.TrimSpace(receipt.RoundID),
		MessageID:                strings.TrimSpace(receipt.MessageID),
		Channel:                  strings.TrimSpace(receipt.Channel),
		Target:                   strings.TrimSpace(receipt.Target),
		ThreadID:                 strings.TrimSpace(receipt.ThreadID),
		PrimaryPlatformMessageID: strings.TrimSpace(receipt.PrimaryPlatformMessageID),
		PlatformMessageIDs:       normalizeStringList(receipt.PlatformMessageIDs),
		Timestamp:                receipt.Timestamp,
	}
	if normalized.PrimaryPlatformMessageID != "" {
		normalized.PlatformMessageIDs = prependUniqueString(
			normalized.PrimaryPlatformMessageID,
			normalized.PlatformMessageIDs,
		)
	}
	return normalized
}

func (r ExternalDeliveryReceipt) hasAddress() bool {
	return r.RoundID != "" || r.MessageID != ""
}

func (r ExternalDeliveryReceipt) hasDeliveryData() bool {
	return r.Channel != "" ||
		r.Target != "" ||
		r.ThreadID != "" ||
		r.PrimaryPlatformMessageID != "" ||
		len(r.PlatformMessageIDs) > 0
}

func externalDeliveryReceiptMessageID(receipt ExternalDeliveryReceipt, timestamp time.Time) string {
	parts := []string{
		"external_delivery_receipt",
		receipt.RoundID,
		receipt.MessageID,
		receipt.Channel,
		receipt.Target,
		receipt.PrimaryPlatformMessageID,
	}
	if receipt.PrimaryPlatformMessageID == "" {
		parts = append(parts, fmt.Sprintf("%d", timestamp.UnixNano()))
	}
	return strings.Join(parts, ":")
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func prependUniqueString(value string, values []string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return values
	}
	result := make([]string, 0, len(values)+1)
	result = append(result, trimmed)
	for _, item := range values {
		if item == trimmed {
			continue
		}
		result = append(result, item)
	}
	return result
}
