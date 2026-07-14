// INPUT: append-only input queue 行及其单项/批量动作。
// OUTPUT: 按顺序重放后的当前队列快照。
// POS: InputQueueStore 的持久日志状态机。
package workspace

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type inputQueueReplayHandler func(*inputQueueReplay, map[string]any)

var inputQueueReplayHandlers = map[string]inputQueueReplayHandler{
	inputQueueActionEnqueue:  (*inputQueueReplay).enqueue,
	inputQueueActionDelete:   (*inputQueueReplay).remove,
	inputQueueActionDispatch: (*inputQueueReplay).remove,
	inputQueueActionReorder:  (*inputQueueReplay).reorder,
	inputQueueActionUpdate:   (*inputQueueReplay).update,
}

func replayInputQueueRows(
	location InputQueueLocation,
	rows []map[string]any,
) []protocol.InputQueueItem {
	replay := newInputQueueReplay(location)
	for _, row := range rows {
		replay.apply(row)
	}
	return replay.items()
}

// inputQueueReplay 维护日志回放期间的实体索引与展示顺序。
type inputQueueReplay struct {
	location  InputQueueLocation
	itemsByID map[string]protocol.InputQueueItem
	order     []string
}

func newInputQueueReplay(location InputQueueLocation) *inputQueueReplay {
	return &inputQueueReplay{
		location:  location,
		itemsByID: make(map[string]protocol.InputQueueItem),
	}
}

func (r *inputQueueReplay) apply(row map[string]any) {
	if handler := inputQueueReplayHandlers[stringFromAny(row["action"])]; handler != nil {
		handler(r, row)
	}
}

func (r *inputQueueReplay) enqueue(row map[string]any) {
	item, ok := inputQueueItemFromAny(row["item"])
	if !ok || strings.TrimSpace(item.ID) == "" || !protocol.HasChatInput(item.Content, item.Attachments) {
		return
	}
	item = normalizeInputQueueItem(r.location, item, normalizeInputQueueTimestamp(row["timestamp"]))
	if _, exists := r.itemsByID[item.ID]; !exists {
		r.order = append(r.order, item.ID)
	}
	r.itemsByID[item.ID] = item
}

func (r *inputQueueReplay) remove(row map[string]any) {
	itemIDs := stringSliceFromAny(row["item_ids"])
	if itemID := stringFromAny(row["item_id"]); itemID != "" {
		itemIDs = append(itemIDs, itemID)
	}
	for _, itemID := range itemIDs {
		delete(r.itemsByID, itemID)
		r.order = removeInputQueueOrderID(r.order, itemID)
	}
}

func (r *inputQueueReplay) reorder(row map[string]any) {
	orderedIDs := stringSliceFromAny(row["ordered_ids"])
	r.order = reorderInputQueueIDs(r.order, r.itemsByID, orderedIDs)
	applyInputQueueOrder(r.itemsByID, orderedIDs, normalizeInputQueueTimestamp(row["timestamp"]))
}

func (r *inputQueueReplay) update(row map[string]any) {
	item, ok := inputQueueItemFromAny(row["item"])
	if !ok || strings.TrimSpace(item.ID) == "" {
		return
	}
	previous, exists := r.itemsByID[item.ID]
	if !exists {
		return
	}
	if item.QueueOrder == 0 {
		item.QueueOrder = previous.QueueOrder
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = previous.CreatedAt
	}
	item = normalizeInputQueueItem(r.location, item, normalizeInputQueueTimestamp(row["timestamp"]))
	r.itemsByID[item.ID] = item
}

func (r *inputQueueReplay) items() []protocol.InputQueueItem {
	result := make([]protocol.InputQueueItem, 0, len(r.order))
	for _, id := range r.order {
		item, ok := r.itemsByID[id]
		if !ok {
			continue
		}
		result = append(result, item)
	}
	return result
}
