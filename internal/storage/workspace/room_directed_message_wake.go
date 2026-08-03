// INPUT: 需要跨进程保留的 Room directed message 延迟唤醒。
// OUTPUT: append-only 的 schedule / complete 日志与待执行快照。
// POS: Room 延迟唤醒的 durable boundary。
package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomWakeActionSchedule = "schedule"
	roomWakeActionComplete = "complete"
)

// RoomDirectedMessageWake 表示一条可恢复的延迟唤醒。
type RoomDirectedMessageWake struct {
	WakeID      string                             `json:"wake_id"`
	OwnerUserID string                             `json:"owner_user_id,omitempty"`
	Message     protocol.RoomDirectedMessageRecord `json:"message"`
	DueAt       int64                              `json:"due_at"`
	CreatedAt   int64                              `json:"created_at"`
}

// RoomDirectedMessageWakeStore 负责延迟唤醒的 append-only 持久化。
type RoomDirectedMessageWakeStore struct {
	paths *Store
	files *SessionFileStore
	mu    sync.Mutex
}

// NewRoomDirectedMessageWakeStore 创建延迟唤醒存储。
func NewRoomDirectedMessageWakeStore(root string) *RoomDirectedMessageWakeStore {
	paths := New(root)
	return &RoomDirectedMessageWakeStore{paths: paths, files: newSessionFileStore(paths)}
}

// Schedule 在返回前将唤醒写入磁盘。
func (s *RoomDirectedMessageWakeStore) Schedule(wake RoomDirectedMessageWake) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	wake.WakeID = strings.TrimSpace(wake.WakeID)
	if wake.WakeID == "" {
		return errors.New("wake_id is required")
	}
	if wake.CreatedAt == 0 {
		wake.CreatedAt = time.Now().UnixMilli()
	}
	return s.files.appendRoomJSONL(wake.OwnerUserID, s.paths.RoomDirectedMessageWakesPath(wake.OwnerUserID), map[string]any{
		"action":    roomWakeActionSchedule,
		"wake":      wake,
		"timestamp": time.Now().UnixMilli(),
	})
}

// Complete 记录唤醒已成功交给运行队列。
func (s *RoomDirectedMessageWakeStore) Complete(ownerUserID string, wakeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	wakeID = strings.TrimSpace(wakeID)
	if wakeID == "" {
		return nil
	}
	return s.files.appendRoomJSONL(ownerUserID, s.paths.RoomDirectedMessageWakesPath(ownerUserID), map[string]any{
		"action":    roomWakeActionComplete,
		"wake_id":   wakeID,
		"timestamp": time.Now().UnixMilli(),
	})
}

// Pending 重放日志并返回尚未完成的唤醒。
func (s *RoomDirectedMessageWakeStore) Pending(ownerUserID string) ([]RoomDirectedMessageWake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingLocked(ownerUserID)
}

// PendingAll 逐用户重放唤醒日志，供进程启动恢复使用。
func (s *RoomDirectedMessageWakeStore) PendingAll() ([]RoomDirectedMessageWake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owners, err := listRoomOwnerPathSegments(s.paths.UsersRoot)
	if err != nil {
		return nil, err
	}
	result := make([]RoomDirectedMessageWake, 0)
	for _, ownerPathSegment := range owners {
		pending, pendingErr := s.pendingPathSegmentLocked(ownerPathSegment)
		if pendingErr != nil {
			return nil, pendingErr
		}
		result = append(result, pending...)
	}
	sortRoomDirectedMessageWakes(result)
	return result, nil
}

func (s *RoomDirectedMessageWakeStore) pendingLocked(ownerUserID string) ([]RoomDirectedMessageWake, error) {
	return s.pendingRowsLocked(ownerUserID, "")
}

func (s *RoomDirectedMessageWakeStore) pendingPathSegmentLocked(
	ownerPathSegment string,
) ([]RoomDirectedMessageWake, error) {
	return s.pendingRowsLocked(ownerPathSegment, ownerPathSegment)
}

func (s *RoomDirectedMessageWakeStore) pendingRowsLocked(
	pathOwner string,
	ownerPathSegment string,
) ([]RoomDirectedMessageWake, error) {
	rows, err := s.files.readRoomJSONL(pathOwner, s.paths.RoomDirectedMessageWakesPath(pathOwner))
	if errors.Is(err, os.ErrNotExist) {
		return []RoomDirectedMessageWake{}, nil
	}
	if err != nil {
		return nil, err
	}
	pending := make(map[string]RoomDirectedMessageWake)
	for _, row := range rows {
		switch strings.TrimSpace(stringFromAny(row["action"])) {
		case roomWakeActionSchedule:
			payload, marshalErr := json.Marshal(row["wake"])
			if marshalErr != nil {
				continue
			}
			var wake RoomDirectedMessageWake
			if json.Unmarshal(payload, &wake) != nil || strings.TrimSpace(wake.WakeID) == "" {
				continue
			}
			if ownerPathSegment == "" {
				wake.OwnerUserID = strings.TrimSpace(pathOwner)
			} else {
				recoveredOwnerUserID, ok := roomLedgerOwnerUserID(
					ownerPathSegment,
					wake.OwnerUserID,
				)
				if !ok {
					continue
				}
				wake.OwnerUserID = recoveredOwnerUserID
			}
			pending[wake.WakeID] = wake
		case roomWakeActionComplete:
			delete(pending, strings.TrimSpace(stringFromAny(row["wake_id"])))
		}
	}
	result := make([]RoomDirectedMessageWake, 0, len(pending))
	for _, wake := range pending {
		result = append(result, wake)
	}
	sortRoomDirectedMessageWakes(result)
	return result, nil
}

func sortRoomDirectedMessageWakes(values []RoomDirectedMessageWake) {
	sort.Slice(values, func(i int, j int) bool {
		if values[i].DueAt != values[j].DueAt {
			return values[i].DueAt < values[j].DueAt
		}
		return values[i].WakeID < values[j].WakeID
	})
}
