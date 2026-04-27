package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	for _, rootRoundID := range rootRoundIDs {
		if itemRootRoundID == strings.TrimSpace(rootRoundID) {
			return true
		}
	}
	return false
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

func (s *InputQueueStore) removeLocked(
	location InputQueueLocation,
	itemID string,
	action string,
) ([]protocol.InputQueueItem, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return s.snapshotLocked(location)
	}
	if err := s.appendActionLocked(location, map[string]any{
		"action":    action,
		"item_id":   itemID,
		"timestamp": time.Now().UnixMilli(),
	}); err != nil {
		return nil, err
	}
	return s.snapshotLocked(location)
}

func (s *InputQueueStore) snapshotLocked(location InputQueueLocation) ([]protocol.InputQueueItem, error) {
	path, err := s.pathForLocation(location)
	if err != nil {
		return nil, err
	}
	rows, err := s.files.readJSONL(path)
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.InputQueueItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	return replayInputQueueRows(location, rows), nil
}

func (s *InputQueueStore) appendActionLocked(location InputQueueLocation, row map[string]any) error {
	path, err := s.pathForLocation(location)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return s.files.appendJSONL(path, row)
}

func (s *InputQueueStore) pathForLocation(location InputQueueLocation) (string, error) {
	workspacePath := strings.TrimSpace(location.WorkspacePath)
	sessionKey := strings.TrimSpace(location.SessionKey)
	if workspacePath == "" {
		return "", errors.New("workspace_path is required")
	}
	if sessionKey == "" {
		return "", errors.New("session_key is required")
	}
	return s.paths.SessionInputQueuePath(workspacePath, sessionKey), nil
}

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
			if !ok || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Content) == "" {
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

func normalizeInputQueueItem(
	location InputQueueLocation,
	item protocol.InputQueueItem,
	now int64,
) protocol.InputQueueItem {
	item.ID = strings.TrimSpace(item.ID)
	item.Scope = protocol.NormalizeInputQueueScope(string(firstNonEmpty(string(item.Scope), string(location.Scope))))
	item.SessionKey = strings.TrimSpace(firstNonEmpty(item.SessionKey, location.SessionKey))
	item.RoomID = strings.TrimSpace(firstNonEmpty(item.RoomID, location.RoomID))
	item.ConversationID = strings.TrimSpace(firstNonEmpty(item.ConversationID, location.ConversationID))
	item.AgentID = strings.TrimSpace(item.AgentID)
	item.SourceAgentID = strings.TrimSpace(item.SourceAgentID)
	item.SourceMessageID = strings.TrimSpace(item.SourceMessageID)
	item.TargetAgentIDs = normalizeInputQueueTargets(item.TargetAgentIDs)
	item.Source = protocol.NormalizeInputQueueSource(string(item.Source))
	item.Content = strings.TrimSpace(item.Content)
	item.DeliveryPolicy = protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy))
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.RootRoundID = strings.TrimSpace(item.RootRoundID)
	if item.HopIndex < 0 {
		item.HopIndex = 0
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = item.CreatedAt
	}
	if item.QueueOrder == 0 {
		item.QueueOrder = item.CreatedAt
	}
	return item
}

func inputQueueItemFromAny(value any) (protocol.InputQueueItem, bool) {
	payload, err := json.Marshal(value)
	if err != nil {
		return protocol.InputQueueItem{}, false
	}
	var item protocol.InputQueueItem
	if err = json.Unmarshal(payload, &item); err != nil {
		return protocol.InputQueueItem{}, false
	}
	return item, true
}

func normalizeInputQueueTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeInputQueueTimestamp(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return time.Now().UnixMilli()
	}
}

func stringSliceFromAny(value any) []string {
	rawItems, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text := stringFromAny(item)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func removeInputQueueOrderID(order []string, itemID string) []string {
	result := order[:0]
	for _, id := range order {
		if id == itemID {
			continue
		}
		result = append(result, id)
	}
	return result
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
