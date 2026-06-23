package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func replayInputQueueRows(
	location InputQueueLocation,
	rows []map[string]any,
) []protocol.InputQueueItem {
	itemsByID := make(map[string]protocol.InputQueueItem)
	order := make([]string, 0)

	for _, row := range rows {
		action := stringFromAny(row["action"])
		switch action {
		case inputQueueActionEnqueue:
			item, ok := inputQueueItemFromAny(row["item"])
			if !ok || strings.TrimSpace(item.ID) == "" || !protocol.HasChatInput(item.Content, item.Attachments) {
				continue
			}
			item = normalizeInputQueueItem(location, item, normalizeInputQueueTimestamp(row["timestamp"]))
			if _, exists := itemsByID[item.ID]; !exists {
				order = append(order, item.ID)
			}
			itemsByID[item.ID] = item
		case inputQueueActionDelete, inputQueueActionDispatch:
			itemID := stringFromAny(row["item_id"])
			if itemID == "" {
				continue
			}
			delete(itemsByID, itemID)
			order = removeInputQueueOrderID(order, itemID)
		case inputQueueActionReorder:
			orderedIDs := stringSliceFromAny(row["ordered_ids"])
			order = reorderInputQueueIDs(order, itemsByID, orderedIDs)
			applyInputQueueOrder(itemsByID, orderedIDs, normalizeInputQueueTimestamp(row["timestamp"]))
		case inputQueueActionUpdate:
			item, ok := inputQueueItemFromAny(row["item"])
			if !ok || strings.TrimSpace(item.ID) == "" {
				continue
			}
			if _, exists := itemsByID[item.ID]; !exists {
				continue
			}
			previous := itemsByID[item.ID]
			item = normalizeInputQueueItem(location, item, normalizeInputQueueTimestamp(row["timestamp"]))
			if item.QueueOrder == 0 {
				item.QueueOrder = previous.QueueOrder
			}
			if item.CreatedAt == 0 {
				item.CreatedAt = previous.CreatedAt
			}
			itemsByID[item.ID] = item
		}
	}

	result := make([]protocol.InputQueueItem, 0, len(order))
	for _, id := range order {
		item, ok := itemsByID[id]
		if !ok {
			continue
		}
		result = append(result, item)
	}
	return result
}
