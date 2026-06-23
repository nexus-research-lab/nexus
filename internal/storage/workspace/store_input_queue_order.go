package workspace

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func removeInputQueueOrderID(order []string, itemID string) []string {
	return slices.DeleteFunc(order, func(id string) bool {
		return id == itemID
	})
}

func reorderInputQueueIDs(
	current []string,
	itemsByID map[string]protocol.InputQueueItem,
	orderedIDs []string,
) []string {
	result := make([]string, 0, len(current))
	seen := make(map[string]struct{}, len(current))
	for _, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := itemsByID[id]; !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	for _, id := range current {
		if _, ok := itemsByID[id]; !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func applyInputQueueOrder(itemsByID map[string]protocol.InputQueueItem, orderedIDs []string, timestamp int64) {
	for index, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		item, ok := itemsByID[id]
		if !ok {
			continue
		}
		item.QueueOrder = timestamp + int64(index)
		item.UpdatedAt = item.QueueOrder
		itemsByID[id] = item
	}
}
