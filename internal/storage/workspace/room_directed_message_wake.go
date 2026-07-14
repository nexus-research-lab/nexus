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
	return &RoomDirectedMessageWakeStore{paths: New(root), files: NewSessionFileStore(root)}
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
	return s.files.appendJSONL(s.paths.RoomDirectedMessageWakesPath(), map[string]any{
		"action":    roomWakeActionSchedule,
		"wake":      wake,
		"timestamp": time.Now().UnixMilli(),
	})
}

// Complete 记录唤醒已成功交给运行队列。
func (s *RoomDirectedMessageWakeStore) Complete(wakeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	wakeID = strings.TrimSpace(wakeID)
	if wakeID == "" {
		return nil
	}
	return s.files.appendJSONL(s.paths.RoomDirectedMessageWakesPath(), map[string]any{
		"action":    roomWakeActionComplete,
		"wake_id":   wakeID,
		"timestamp": time.Now().UnixMilli(),
	})
}

// Pending 重放日志并返回尚未完成的唤醒。
func (s *RoomDirectedMessageWakeStore) Pending() ([]RoomDirectedMessageWake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.files.readJSONL(s.paths.RoomDirectedMessageWakesPath())
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
			pending[wake.WakeID] = wake
		case roomWakeActionComplete:
			delete(pending, strings.TrimSpace(stringFromAny(row["wake_id"])))
		}
	}
	result := make([]RoomDirectedMessageWake, 0, len(pending))
	for _, wake := range pending {
		result = append(result, wake)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].DueAt != result[j].DueAt {
			return result[i].DueAt < result[j].DueAt
		}
		return result[i].WakeID < result[j].WakeID
	})
	return result, nil
}
