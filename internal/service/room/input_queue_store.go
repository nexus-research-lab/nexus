package room

import (
	"context"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) roomInputQueueItems(ctx context.Context, contextValue *protocol.ConversationContextAggregate) ([]protocol.InputQueueItem, error) {
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	items := make([]protocol.InputQueueItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Item)
	}
	return items, nil
}

func (s *RealtimeService) roomInputQueueEntries(ctx context.Context, contextValue *protocol.ConversationContextAggregate) ([]roomInputQueueEntry, error) {
	locations, err := s.roomInputQueueLocations(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	entries := make([]roomInputQueueEntry, 0)
	for _, location := range locations {
		items, snapshotErr := s.inputQueue.Snapshot(location.Location)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		for _, item := range items {
			entries = append(entries, roomInputQueueEntry{
				Item:     item,
				Location: location.Location,
			})
		}
	}
	sort.SliceStable(entries, func(i int, j int) bool {
		left := entries[i].Item
		right := entries[j].Item
		if left.QueueOrder != right.QueueOrder {
			return left.QueueOrder < right.QueueOrder
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		return left.ID < right.ID
	})
	return entries, nil
}

func (s *RealtimeService) findRoomInputQueueEntry(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	itemID string,
) (roomInputQueueEntry, bool, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return roomInputQueueEntry{}, false, nil
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return roomInputQueueEntry{}, false, err
	}
	for _, entry := range entries {
		if entry.Item.ID == itemID {
			return entry, true, nil
		}
	}
	return roomInputQueueEntry{}, false, nil
}

func (s *RealtimeService) deleteRoomInputQueueItem(ctx context.Context, contextValue *protocol.ConversationContextAggregate, itemID string) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil || !ok {
		return err
	}
	_, err = s.inputQueue.Delete(entry.Location, itemID)
	return err
}

func (s *RealtimeService) reorderRoomInputQueueItems(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	orderedIDs []string,
) error {
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return err
	}
	locationByKey := make(map[string]workspacestore.InputQueueLocation)
	for _, entry := range entries {
		for _, orderedID := range orderedIDs {
			if entry.Item.ID != strings.TrimSpace(orderedID) {
				continue
			}
			locationByKey[inputQueueLocationKey(entry.Location)] = entry.Location
			break
		}
	}
	for _, location := range locationByKey {
		if _, err = s.inputQueue.Reorder(location, orderedIDs); err != nil {
			return err
		}
	}
	return nil
}
