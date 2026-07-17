// INPUT: DM / Room 待发送项、客户端幂等键、投递策略与预检过的 guidance 语义版本。
// OUTPUT: append-only 队列快照、持久接受结果、串行派发和忽略排序时间戳的全有或全无 guidance 认领。
// POS: 会话输入队列的持久化真相源；业务服务不得自行删除已预检项。
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// ErrInputQueueCapacity 表示目标队列已达安全上限。
var ErrInputQueueCapacity = errors.New("input queue capacity exceeded")

// ErrInputQueueIdempotencyConflict 表示同一客户端幂等键被用于不同的入队语义。
var ErrInputQueueIdempotencyConflict = errors.New("input queue idempotency conflict")

// InputQueueLocation 描述待发送队列的物理位置。
type InputQueueLocation struct {
	Scope          protocol.InputQueueScope
	WorkspacePath  string
	SessionKey     string
	RoomID         string
	ConversationID string
}

// InputQueueEnqueue 把一个待登记项与它的物理队列位置绑定。
type InputQueueEnqueue struct {
	Location InputQueueLocation
	Item     protocol.InputQueueItem
}

// InputQueueEnqueueResult 表示一次幂等入队的持久结果与当前快照。
type InputQueueEnqueueResult struct {
	Item      protocol.InputQueueItem
	Items     []protocol.InputQueueItem
	Duplicate bool
}

type preparedInputQueueEnqueue struct {
	location InputQueueLocation
	item     protocol.InputQueueItem
	previous *protocol.InputQueueItem
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
	rows, err := s.inputQueueRowsLocked(location)
	if err != nil {
		return nil, err
	}
	return replayInputQueueRows(location, rows), nil
}

func (s *InputQueueStore) inputQueueRowsLocked(location InputQueueLocation) ([]map[string]any, error) {
	path, err := s.pathForLocation(location)
	if err != nil {
		return nil, err
	}
	rows, err := s.files.readJSONL(path)
	if errors.Is(err, os.ErrNotExist) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
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

// FindAcceptedEnqueue 查找此前已持久接受的客户端入队请求。
// 即使队列项已经派发，append-only enqueue 行仍会命中。
func (s *InputQueueStore) FindAcceptedEnqueue(
	location InputQueueLocation,
	clientMessageID string,
) (protocol.InputQueueItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.inputQueueRowsLocked(location)
	if err != nil {
		return protocol.InputQueueItem{}, false, err
	}
	item, ok := findAcceptedInputQueueEnqueue(location, rows, clientMessageID)
	return item, ok, nil
}

// EnqueueIdempotent 按客户端消息 ID 持久接受一次入队。
// 重试命中已接受语义时不追加日志；同键不同语义会返回冲突。
func (s *InputQueueStore) EnqueueIdempotent(
	location InputQueueLocation,
	item protocol.InputQueueItem,
	clientMessageID string,
) (InputQueueEnqueueResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientMessageID = strings.TrimSpace(clientMessageID)
	if clientMessageID == "" {
		return InputQueueEnqueueResult{}, errors.New("client_message_id is required")
	}
	rows, err := s.inputQueueRowsLocked(location)
	if err != nil {
		return InputQueueEnqueueResult{}, err
	}

	now := time.Now().UnixMilli()
	item = normalizeInputQueueItem(location, item, now)
	if existing, ok := findAcceptedInputQueueEnqueue(location, rows, clientMessageID); ok {
		if !MatchesInputQueueEnqueueIntent(existing, item) {
			return InputQueueEnqueueResult{}, fmt.Errorf(
				"%w: client_message_id %q",
				ErrInputQueueIdempotencyConflict,
				clientMessageID,
			)
		}
		return InputQueueEnqueueResult{
			Item:      existing,
			Items:     replayInputQueueRows(location, rows),
			Duplicate: true,
		}, nil
	}

	if item.ID == "" {
		item.ID = NewInputQueueID()
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	row := map[string]any{
		"action":            inputQueueActionEnqueue,
		"item":              item,
		"client_message_id": clientMessageID,
		"timestamp":         now,
	}
	if err = s.appendActionLocked(location, row); err != nil {
		return InputQueueEnqueueResult{}, err
	}

	// append 成功即为 durable acceptance；快照直接重放已读行 + 新行，
	// 不在提交后执行第二次可失败 I/O。
	rows = append(rows, row)
	return InputQueueEnqueueResult{
		Item:  item,
		Items: replayInputQueueRows(location, rows),
	}, nil
}

func findAcceptedInputQueueEnqueue(
	location InputQueueLocation,
	rows []map[string]any,
	clientMessageID string,
) (protocol.InputQueueItem, bool) {
	clientMessageID = strings.TrimSpace(clientMessageID)
	if clientMessageID == "" {
		return protocol.InputQueueItem{}, false
	}
	for _, row := range rows {
		if stringFromAny(row["action"]) != inputQueueActionEnqueue ||
			stringFromAny(row["client_message_id"]) != clientMessageID {
			continue
		}
		item, ok := inputQueueItemFromAny(row["item"])
		if !ok || strings.TrimSpace(item.ID) == "" {
			continue
		}
		return normalizeInputQueueItem(location, item, normalizeInputQueueTimestamp(row["timestamp"])), true
	}
	return protocol.InputQueueItem{}, false
}

// MatchesInputQueueEnqueueIntent 判断两条记录是否表达同一逻辑入队意图。
// 比较忽略队列 ID、时间戳与 agent session 等物理落点。
func MatchesInputQueueEnqueueIntent(existing protocol.InputQueueItem, candidate protocol.InputQueueItem) bool {
	return reflect.DeepEqual(
		normalizeInputQueueEnqueueIntent(existing),
		normalizeInputQueueEnqueueIntent(candidate),
	)
}

type inputQueueEnqueueIntent struct {
	Content        string
	Attachments    []protocol.ChatAttachment
	DeliveryPolicy protocol.ChatDeliveryPolicy
	TargetAgentIDs []string
	Source         protocol.InputQueueSource
	OwnerUserID    string
}

func normalizeInputQueueEnqueueIntent(item protocol.InputQueueItem) inputQueueEnqueueIntent {
	targetAgentIDs := normalizeInputQueueTargets(item.TargetAgentIDs)
	targetAgentIDs = slices.Clone(targetAgentIDs)
	slices.Sort(targetAgentIDs)
	return inputQueueEnqueueIntent{
		Content:        strings.TrimSpace(item.Content),
		Attachments:    protocol.NormalizeChatAttachments(item.Attachments, ""),
		DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		TargetAgentIDs: targetAgentIDs,
		Source:         protocol.NormalizeInputQueueSource(string(item.Source)),
		OwnerUserID:    strings.TrimSpace(item.OwnerUserID),
	}
}

// EnqueueBounded 在同一把锁内完成去重、容量检查与入队。
func (s *InputQueueStore) EnqueueBounded(
	location InputQueueLocation,
	item protocol.InputQueueItem,
	capacity int,
) ([]protocol.InputQueueItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	item = normalizeInputQueueItem(location, item, now)
	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, false, err
	}
	for _, existing := range items {
		if existing.Source == item.Source &&
			strings.TrimSpace(existing.SourceMessageID) == strings.TrimSpace(item.SourceMessageID) &&
			strings.TrimSpace(existing.AgentID) == strings.TrimSpace(item.AgentID) {
			return items, false, nil
		}
	}
	if capacity > 0 && len(items) >= capacity {
		return items, false, ErrInputQueueCapacity
	}
	if item.ID == "" {
		item.ID = NewInputQueueID()
	}
	item.CreatedAt = now
	item.UpdatedAt = now
	if err = s.appendActionLocked(location, map[string]any{
		"action": inputQueueActionEnqueue, "item": item, "timestamp": now,
	}); err != nil {
		return nil, false, err
	}
	items, err = s.snapshotLocked(location)
	return items, true, err
}

// EnqueueBatch 预检并登记整批队列项；写入失败时恢复本次写入前的状态。
func (s *InputQueueStore) EnqueueBatch(entries []InputQueueEnqueue) error {
	_, err := s.EnqueueBatchWithItems(entries)
	return err
}

// EnqueueBatchWithItems 原子登记整批队列项，并返回实际提交的规范化版本。
// 调用方可直接用返回值继续做 prepared CAS，无需在恢复后再次读取快照。
func (s *InputQueueStore) EnqueueBatchWithItems(entries []InputQueueEnqueue) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	prepared := make([]preparedInputQueueEnqueue, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		path, err := s.pathForLocation(entry.Location)
		if err != nil {
			return nil, err
		}
		item := normalizeInputQueueItem(entry.Location, entry.Item, now)
		if item.ID == "" {
			item.ID = NewInputQueueID()
		}
		key := path + "\x00" + item.ID
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate input queue item in batch: %s", item.ID)
		}
		seen[key] = struct{}{}
		if item.CreatedAt == 0 {
			item.CreatedAt = now
		}
		item.UpdatedAt = now

		current, err := s.snapshotLocked(entry.Location)
		if err != nil {
			return nil, err
		}
		var previous *protocol.InputQueueItem
		for _, existing := range current {
			if existing.ID == item.ID {
				copyItem := existing
				previous = &copyItem
				break
			}
		}
		prepared = append(prepared, preparedInputQueueEnqueue{location: entry.Location, item: item, previous: previous})
	}

	committed := make([]preparedInputQueueEnqueue, 0, len(prepared))
	for _, entry := range prepared {
		if err := s.appendActionLocked(entry.location, map[string]any{
			"action":    inputQueueActionEnqueue,
			"item":      entry.item,
			"timestamp": now,
		}); err != nil {
			rollbackErr := s.rollbackEnqueueBatchLocked(committed, now)
			return nil, errors.Join(err, rollbackErr)
		}
		committed = append(committed, entry)
	}
	items := make([]protocol.InputQueueItem, 0, len(committed))
	for _, entry := range committed {
		items = append(items, entry.item)
	}
	return items, nil
}

func (s *InputQueueStore) rollbackEnqueueBatchLocked(entries []preparedInputQueueEnqueue, now int64) error {
	var rollbackErr error
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		row := map[string]any{
			"action":    inputQueueActionDelete,
			"item_id":   entry.item.ID,
			"timestamp": now,
		}
		if entry.previous != nil {
			row = map[string]any{
				"action":    inputQueueActionEnqueue,
				"item":      *entry.previous,
				"timestamp": now,
			}
		}
		rollbackErr = errors.Join(rollbackErr, s.appendActionLocked(entry.location, row))
	}
	return rollbackErr
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

// DispatchMany 原子弹出一批已合并为同一轮的队列项。
func (s *InputQueueStore) DispatchMany(
	location InputQueueLocation,
	itemIDs []string,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if err := s.appendActionLocked(location, map[string]any{
			"action": inputQueueActionDispatch, "item_id": itemID, "timestamp": now,
		}); err != nil {
			return nil, err
		}
	}
	return s.snapshotLocked(location)
}

// SnapshotGuidance 返回当前可由指定 round 消费的引导项，但不改变队列。
func (s *InputQueueStore) SnapshotGuidance(
	location InputQueueLocation,
	rootRoundIDs ...string,
) ([]protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, err
	}
	return matchingGuidanceItems(items, rootRoundIDs), nil
}

// DispatchPreparedGuidance 仅在预检项仍保持原版本时原子消费整批引导。
// 任一项已变化时不消费任何内容，让调用方在下一次 hook 重新预检。
func (s *InputQueueStore) DispatchPreparedGuidance(
	location InputQueueLocation,
	prepared []protocol.InputQueueItem,
	rootRoundIDs ...string,
) ([]protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.snapshotLocked(location)
	if err != nil {
		return nil, nil, err
	}
	if len(prepared) == 0 {
		return nil, items, nil
	}
	currentByID := make(map[string]protocol.InputQueueItem, len(items))
	for _, item := range items {
		currentByID[item.ID] = item
	}
	claimed := make([]protocol.InputQueueItem, 0, len(prepared))
	for _, expected := range prepared {
		current, ok := currentByID[expected.ID]
		if !ok || !samePreparedGuidanceItem(current, expected) ||
			!protocol.ShouldGuideRunningRound(current.DeliveryPolicy) ||
			!matchesInputQueueGuidanceTarget(current, rootRoundIDs) {
			return nil, items, nil
		}
		claimed = append(claimed, current)
	}
	return s.dispatchGuidanceItemsLocked(location, items, claimed)
}

// DispatchGuidance 原子弹出所有等待 hook 引导的队列项。
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
	guidanceItems := matchingGuidanceItems(items, rootRoundIDs)
	if len(guidanceItems) == 0 {
		return nil, items, nil
	}
	return s.dispatchGuidanceItemsLocked(location, items, guidanceItems)
}

func matchingGuidanceItems(items []protocol.InputQueueItem, rootRoundIDs []string) []protocol.InputQueueItem {
	guidanceItems := make([]protocol.InputQueueItem, 0)
	for _, item := range items {
		if protocol.ShouldGuideRunningRound(item.DeliveryPolicy) && matchesInputQueueGuidanceTarget(item, rootRoundIDs) {
			guidanceItems = append(guidanceItems, item)
		}
	}
	return guidanceItems
}

func samePreparedGuidanceItem(current protocol.InputQueueItem, expected protocol.InputQueueItem) bool {
	current.QueueOrder = 0
	current.UpdatedAt = 0
	expected.QueueOrder = 0
	expected.UpdatedAt = 0
	return reflect.DeepEqual(current, expected)
}

func (s *InputQueueStore) dispatchGuidanceItemsLocked(
	location InputQueueLocation,
	currentItems []protocol.InputQueueItem,
	guidanceItems []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	now := time.Now().UnixMilli()
	itemIDs := make([]string, 0, len(guidanceItems))
	dispatchedIDs := make(map[string]struct{}, len(guidanceItems))
	for _, item := range guidanceItems {
		itemIDs = append(itemIDs, item.ID)
		dispatchedIDs[item.ID] = struct{}{}
	}
	// next 由已持锁读取的快照计算；dispatch commit 成功后不再做可失败的读，
	// 否则调用方会丢失已经 durable dispatch 的 claimed items，无法回滚。
	next := make([]protocol.InputQueueItem, 0, len(currentItems))
	for _, item := range currentItems {
		if _, dispatched := dispatchedIDs[item.ID]; dispatched {
			continue
		}
		next = append(next, item)
	}
	if err := s.appendActionLocked(location, map[string]any{
		"action":    inputQueueActionDispatch,
		"item_ids":  itemIDs,
		"timestamp": now,
	}); err != nil {
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
