package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	inputQueueActionEnqueue  = "enqueue"
	inputQueueActionDelete   = "delete"
	inputQueueActionDispatch = "dispatch"
	inputQueueActionReorder  = "reorder"
	inputQueueActionUpdate   = "update"
)

// InputQueueLocation 描述待发送队列的物理位置。
type InputQueueLocation struct {
	Scope          protocol.InputQueueScope
	WorkspacePath  string
	SessionKey     string
	RoomID         string
	ConversationID string
}

// InputQueueStore 负责待发送队列的 append-only JSONL 存储。
type InputQueueStore struct {
	paths *Store
	files *SessionFileStore
	mu    sync.Mutex
}

// NewInputQueueStore 创建待发送队列存储。
func NewInputQueueStore(root string) *InputQueueStore {
	return &InputQueueStore{
		paths: New(root),
		files: NewSessionFileStore(root),
	}
}

// NewInputQueueID 生成待发送队列项 ID。
func NewInputQueueID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("input_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

// Snapshot 读取当前位置的待发送队列快照。
func (s *InputQueueStore) Snapshot(location InputQueueLocation) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked(location)
}

// Enqueue 追加一条待发送队列项并返回最新快照。
func (s *InputQueueStore) Enqueue(
	location InputQueueLocation,
	item protocol.InputQueueItem,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	item = normalizeInputQueueItem(location, item, now)
	if item.ID == "" {
		item.ID = NewInputQueueID()
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	if err := s.appendActionLocked(location, map[string]any{
		"action":    inputQueueActionEnqueue,
		"item":      item,
		"timestamp": now,
	}); err != nil {
		return nil, err
	}
	return s.snapshotLocked(location)
}

// Delete 删除指定队列项并返回最新快照。
func (s *InputQueueStore) Delete(location InputQueueLocation, itemID string) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeLocked(location, itemID, inputQueueActionDelete)
}

// UpdateDeliveryPolicy 更新队列项投递策略，并返回最新快照。
func (s *InputQueueStore) UpdateDeliveryPolicy(
	location InputQueueLocation,
	itemID string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
	rootRoundIDs ...string,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return items, nil
	}
	var selected *protocol.InputQueueItem
	for _, item := range items {
		if item.ID != itemID {
			continue
		}
		copyItem := item
		selected = &copyItem
		break
	}
	if selected == nil {
		return items, nil
	}

	now := time.Now().UnixMilli()
	selected.DeliveryPolicy = protocol.NormalizeChatDeliveryPolicy(string(deliveryPolicy))
	if len(rootRoundIDs) > 0 {
		selected.RootRoundID = strings.TrimSpace(rootRoundIDs[0])
	} else {
		selected.RootRoundID = ""
	}
	selected.UpdatedAt = now
	if err = s.appendActionLocked(location, map[string]any{
		"action":    inputQueueActionUpdate,
		"item":      *selected,
		"timestamp": now,
	}); err != nil {
		return nil, err
	}
	return s.snapshotLocked(location)
}

// DispatchNext 弹出队首项，追加派发事件，并返回队首项与最新快照。
func (s *InputQueueStore) DispatchNext(
	location InputQueueLocation,
) (*protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		return nil, items, nil
	}
	item := items[0]
	next, err := s.removeLocked(location, item.ID, inputQueueActionDispatch)
	if err != nil {
		return nil, nil, err
	}
	return &item, next, nil
}

// DispatchFirstDispatchable 弹出第一条普通待发送项，guide 项只等待 hook 消费。
func (s *InputQueueStore) DispatchFirstDispatchable(
	location InputQueueLocation,
) (*protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, nil, err
	}
	for _, item := range items {
		if protocol.ShouldGuideRunningRound(item.DeliveryPolicy) {
			continue
		}
		next, err := s.removeLocked(location, item.ID, inputQueueActionDispatch)
		if err != nil {
			return nil, nil, err
		}
		copyItem := item
		return &copyItem, next, nil
	}
	return nil, items, nil
}

// Dispatch 删除指定待发送项，追加派发事件，并返回最新快照。
func (s *InputQueueStore) Dispatch(
	location InputQueueLocation,
	itemID string,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeLocked(location, itemID, inputQueueActionDispatch)
}

// DispatchGuidance 弹出所有等待 hook 引导的队列项。
func (s *InputQueueStore) DispatchGuidance(
	location InputQueueLocation,
	rootRoundIDs ...string,
) ([]protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, nil, err
	}
	guidanceItems := make([]protocol.InputQueueItem, 0)
	for _, item := range items {
		if protocol.ShouldGuideRunningRound(item.DeliveryPolicy) && matchesInputQueueGuidanceTarget(item, rootRoundIDs) {
			guidanceItems = append(guidanceItems, item)
		}
	}
	if len(guidanceItems) == 0 {
		return nil, items, nil
	}
	now := time.Now().UnixMilli()
	for _, item := range guidanceItems {
		if err = s.appendActionLocked(location, map[string]any{
			"action":    inputQueueActionDispatch,
			"item_id":   item.ID,
			"timestamp": now,
		}); err != nil {
			return nil, nil, err
		}
	}
	next, err := s.snapshotLocked(location)
	if err != nil {
		return nil, nil, err
	}
	return guidanceItems, next, nil
}

func matchesInputQueueGuidanceTarget(item protocol.InputQueueItem, rootRoundIDs []string) bool {
	if len(rootRoundIDs) == 0 {
		return true
	}
	itemRootRoundID := strings.TrimSpace(item.RootRoundID)
	if itemRootRoundID == "" {
		return true
	}
	return slices.ContainsFunc(rootRoundIDs, func(rootRoundID string) bool {
		return itemRootRoundID == strings.TrimSpace(rootRoundID)
	})
}

// Reorder 根据 orderedIDs 重排队列，并返回最新快照。
func (s *InputQueueStore) Reorder(
	location InputQueueLocation,
	orderedIDs []string,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanIDs := make([]string, 0, len(orderedIDs))
	seen := make(map[string]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		cleanIDs = append(cleanIDs, id)
	}
	if err := s.appendActionLocked(location, map[string]any{
		"action":      inputQueueActionReorder,
		"ordered_ids": cleanIDs,
		"timestamp":   time.Now().UnixMilli(),
	}); err != nil {
		return nil, err
	}
	return s.snapshotLocked(location)
}
