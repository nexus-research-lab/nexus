package workspace

import (
	"slices"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func splitExternalDeliveryReceipts(rows []protocol.Message) ([]protocol.Message, []ExternalDeliveryReceipt) {
	if len(rows) == 0 {
		return rows, nil
	}
	visibleRows := make([]protocol.Message, 0, len(rows))
	deliveryReceipts := make([]ExternalDeliveryReceipt, 0)
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindExternalDelivery {
			visibleRows = append(visibleRows, row)
			continue
		}
		if receipt := externalDeliveryReceiptFromRow(row); receipt.hasAddress() {
			deliveryReceipts = append(deliveryReceipts, receipt)
		}
	}
	return visibleRows, deliveryReceipts
}

func externalDeliveryReceiptFromRow(row protocol.Message) ExternalDeliveryReceipt {
	return normalizeExternalDeliveryReceipt(ExternalDeliveryReceipt{
		RoundID:                  stringFromAny(row["round_id"]),
		MessageID:                stringFromAny(row["assistant_message_id"]),
		Channel:                  stringFromAny(row["channel"]),
		Target:                   stringFromAny(row["target"]),
		ThreadID:                 stringFromAny(row["thread_id"]),
		PrimaryPlatformMessageID: stringFromAny(row["primary_platform_message_id"]),
		PlatformMessageIDs:       stringSliceFromAny(row["platform_message_ids"]),
		Timestamp:                timeFromUnixMilli(messageTimestamp(row)),
	})
}

func timeFromUnixMilli(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func mergeExternalDeliveryReceipts(
	rows []protocol.Message,
	receipts []ExternalDeliveryReceipt,
) []protocol.Message {
	if len(rows) == 0 || len(receipts) == 0 {
		return rows
	}

	mergedRows := cloneHistoryRows(rows)
	assistantIndexByMessageID := make(map[string]int)
	lastAssistantIndexByRoundID := make(map[string]int)
	for index, row := range mergedRows {
		if protocol.MessageRole(row) != "assistant" {
			continue
		}
		if messageID := stringFromAny(row["message_id"]); messageID != "" {
			assistantIndexByMessageID[messageID] = index
		}
		if roundID := stringFromAny(row["round_id"]); roundID != "" {
			lastAssistantIndexByRoundID[roundID] = index
		}
	}

	for _, receipt := range receipts {
		index, ok := externalDeliveryReceiptAssistantIndex(
			receipt,
			assistantIndexByMessageID,
			lastAssistantIndexByRoundID,
		)
		if !ok {
			continue
		}
		assistant := protocol.Clone(mergedRows[index])
		payload := externalDeliveryReceiptPayload(receipt)
		if len(payload) == 0 {
			continue
		}
		assistant["external_delivery"] = payload
		assistant["external_deliveries"] = appendExternalDeliveryPayload(
			assistant["external_deliveries"],
			payload,
		)
		mergedRows[index] = assistant
	}
	return mergedRows
}

func externalDeliveryReceiptAssistantIndex(
	receipt ExternalDeliveryReceipt,
	assistantIndexByMessageID map[string]int,
	lastAssistantIndexByRoundID map[string]int,
) (int, bool) {
	if receipt.MessageID != "" {
		if index, ok := assistantIndexByMessageID[receipt.MessageID]; ok {
			return index, true
		}
	}
	if receipt.RoundID != "" {
		if index, ok := lastAssistantIndexByRoundID[receipt.RoundID]; ok {
			return index, true
		}
	}
	return 0, false
}

func externalDeliveryReceiptPayload(receipt ExternalDeliveryReceipt) map[string]any {
	payload := make(map[string]any)
	if receipt.Channel != "" {
		payload["channel"] = receipt.Channel
	}
	if receipt.Target != "" {
		payload["target"] = receipt.Target
	}
	if receipt.ThreadID != "" {
		payload["thread_id"] = receipt.ThreadID
	}
	if receipt.PrimaryPlatformMessageID != "" {
		payload["primary_platform_message_id"] = receipt.PrimaryPlatformMessageID
	}
	if len(receipt.PlatformMessageIDs) > 0 {
		payload["platform_message_ids"] = slices.Clone(receipt.PlatformMessageIDs)
	}
	if !receipt.Timestamp.IsZero() {
		payload["delivered_at"] = receipt.Timestamp.UnixMilli()
	}
	return payload
}

func appendExternalDeliveryPayload(value any, payload map[string]any) []map[string]any {
	deliveries := normalizeExternalDeliveryPayloads(value)
	if !hasExternalDeliveryPayload(deliveries, payload) {
		deliveries = append(deliveries, cloneMessageMap(payload))
	}
	return deliveries
}

func normalizeExternalDeliveryPayloads(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, cloneMessageMap(item))
		}
		return result
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				result = append(result, cloneMessageMap(mapped))
			}
		}
		return result
	default:
		return nil
	}
}

func hasExternalDeliveryPayload(deliveries []map[string]any, payload map[string]any) bool {
	channel := stringFromAny(payload["channel"])
	target := stringFromAny(payload["target"])
	primaryPlatformMessageID := stringFromAny(payload["primary_platform_message_id"])
	deliveredAt := messageTimestamp(protocol.Message{"timestamp": payload["delivered_at"]})
	for _, item := range deliveries {
		if stringFromAny(item["channel"]) != channel {
			continue
		}
		if stringFromAny(item["target"]) != target {
			continue
		}
		if stringFromAny(item["primary_platform_message_id"]) != primaryPlatformMessageID {
			continue
		}
		if primaryPlatformMessageID != "" {
			return true
		}
		if deliveredAt != 0 && messageTimestamp(protocol.Message{"timestamp": item["delivered_at"]}) != deliveredAt {
			continue
		}
		return true
	}
	return false
}
