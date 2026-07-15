// INPUT: Room 公区 Agent handoff 的检测、排队、启动与终态事件。
// OUTPUT: 可跨进程恢复且按 handoff_id 幂等的 append-only ledger。
// POS: 公区 handoff 的持久化事实源；InputQueue 只负责 busy 目标的投递。
package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomPublicHandoffActionDetected       = "detected"
	roomPublicHandoffActionSourceFinished = "source_finished"
	roomPublicHandoffActionQueued         = "queued"
	roomPublicHandoffActionClaimed        = "claimed"
	roomPublicHandoffActionStarted        = "started"
	roomPublicHandoffActionTerminal       = "terminal"
	roomPublicHandoffActionCancelled      = "cancelled"
)

// RoomPublicHandoff 表示一条从 source slot 指向 target Agent 的公区协作边。
// 状态迁移通过同一 handoff_id 追加事件，重放后得到最后一个有效状态。
type RoomPublicHandoff struct {
	HandoffID          string                  `json:"handoff_id"`
	ConversationID     string                  `json:"conversation_id"`
	RoomID             string                  `json:"room_id,omitempty"`
	RootRoundID        string                  `json:"root_round_id,omitempty"`
	SourceAgentRoundID string                  `json:"source_agent_round_id,omitempty"`
	SourceMessageID    string                  `json:"source_message_id"`
	SourceAgentID      string                  `json:"source_agent_id"`
	TargetAgentID      string                  `json:"target_agent_id"`
	Content            string                  `json:"content"`
	ReplyRoute         protocol.RoomReplyRoute `json:"reply_route,omitempty"`
	HopIndex           int                     `json:"hop_index,omitempty"`
	QueueItemID        string                  `json:"queue_item_id,omitempty"`
	TargetRoundID      string                  `json:"target_round_id,omitempty"`
	Status             string                  `json:"status"`
	ClaimedAt          int64                   `json:"claimed_at,omitempty"`
	CreatedAt          int64                   `json:"created_at"`
	UpdatedAt          int64                   `json:"updated_at"`
}

// RoomPublicHandoffStore 负责 Room 公区 handoff ledger 的并发追加与重放。
type RoomPublicHandoffStore struct {
	paths *Store
	files *SessionFileStore
	mu    sync.Mutex
}

// NewRoomPublicHandoffStore 创建公区 handoff ledger。
func NewRoomPublicHandoffStore(root string) *RoomPublicHandoffStore {
	return &RoomPublicHandoffStore{paths: New(root), files: NewSessionFileStore(root)}
}

// Detect 记录一条新 handoff；相同 handoff_id 重复检测时保持幂等。
func (s *RoomPublicHandoffStore) Detect(handoff RoomPublicHandoff) (RoomPublicHandoff, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateRoomPublicHandoff(handoff); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	all, err := s.replayLocked(handoff.ConversationID)
	if err != nil {
		return RoomPublicHandoff{}, false, err
	}
	if existing, ok := all[handoff.HandoffID]; ok {
		return existing, false, nil
	}
	now := time.Now().UnixMilli()
	handoff.Status = roomPublicHandoffActionDetected
	if handoff.CreatedAt == 0 {
		handoff.CreatedAt = now
	}
	handoff.UpdatedAt = now
	if err := s.appendLocked(handoff.ConversationID, roomPublicHandoffActionDetected, handoff); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	return handoff, true, nil
}

// MarkSourceFinished 标记 source slot 已成功发布最终消息。
func (s *RoomPublicHandoffStore) MarkSourceFinished(conversationID string, handoffID string) error {
	return s.transition(conversationID, handoffID, roomPublicHandoffActionSourceFinished, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionSourceFinished
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionDetected || value.Status == roomPublicHandoffActionQueued
	})
}

// MarkQueued 记录 handoff 已进入 busy 目标的 InputQueue。
func (s *RoomPublicHandoffStore) MarkQueued(conversationID string, handoffID string, queueItemID string) error {
	return s.transition(conversationID, handoffID, roomPublicHandoffActionQueued, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionQueued
		value.QueueItemID = strings.TrimSpace(queueItemID)
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionDetected ||
			value.Status == roomPublicHandoffActionSourceFinished ||
			value.Status == roomPublicHandoffActionQueued
	})
}

// Claim 为本进程准备启动 handoff，防止恢复器与实时路径重复创建 target round。
func (s *RoomPublicHandoffStore) Claim(conversationID string, handoffID string) (RoomPublicHandoff, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(conversationID)
	if err != nil {
		return RoomPublicHandoff{}, false, err
	}
	value, ok := all[strings.TrimSpace(handoffID)]
	if !ok || !roomPublicHandoffCanStart(value) {
		return value, false, nil
	}
	now := time.Now().UnixMilli()
	value.Status = roomPublicHandoffActionClaimed
	value.ClaimedAt = now
	value.UpdatedAt = now
	if err := s.appendLocked(conversationID, roomPublicHandoffActionClaimed, value); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	return value, true, nil
}

// ReleaseClaim 将启动失败的 handoff 恢复为 source_finished，允许下一次重试。
func (s *RoomPublicHandoffStore) ReleaseClaim(conversationID string, handoffID string) error {
	return s.transition(conversationID, handoffID, roomPublicHandoffActionSourceFinished, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionSourceFinished
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionClaimed
	})
}

// MarkStarted 记录 target round 已创建。
func (s *RoomPublicHandoffStore) MarkStarted(conversationID string, handoffID string, targetRoundID string) error {
	return s.transition(conversationID, handoffID, roomPublicHandoffActionStarted, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionStarted
		value.TargetRoundID = strings.TrimSpace(targetRoundID)
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionClaimed
	})
}

// MarkTerminal 收口 target handoff，status 使用 finished、error 或 interrupted。
func (s *RoomPublicHandoffStore) MarkTerminal(conversationID string, handoffID string, status string) error {
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	return s.transition(conversationID, handoffID, roomPublicHandoffActionTerminal, func(value *RoomPublicHandoff) {
		value.Status = status
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return roomPublicHandoffCanTerminal(value)
	})
}

// CancelForSource 取消尚未启动的 source handoff，防止失败 source 继续唤醒目标。
func (s *RoomPublicHandoffStore) CancelForSource(conversationID string, sourceAgentRoundID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(conversationID)
	if err != nil {
		return err
	}
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	for _, value := range all {
		if strings.TrimSpace(value.SourceAgentRoundID) != strings.TrimSpace(sourceAgentRoundID) ||
			!roomPublicHandoffCanCancelSource(value) {
			continue
		}
		value.Status = status
		value.ClaimedAt = 0
		value.UpdatedAt = time.Now().UnixMilli()
		if err := s.appendLocked(conversationID, roomPublicHandoffActionCancelled, value); err != nil {
			return err
		}
	}
	return nil
}

// Pending 返回需要恢复或观察的 handoff。queued 仍然返回，调用方需要先确认
// 对应 InputQueue item 是否还在；若队列项已丢失，才能把它重新交给实时派发。
func (s *RoomPublicHandoffStore) Pending(conversationID string) ([]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(conversationID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result := make([]RoomPublicHandoff, 0, len(all))
	for _, value := range all {
		if roomPublicHandoffIsPending(value, now) {
			result = append(result, value)
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].HandoffID < result[j].HandoffID
	})
	return result, nil
}

// PendingAll 扫描 workspace 下所有 Room handoff ledger，供进程启动恢复使用。
func (s *RoomPublicHandoffStore) PendingAll() ([]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.paths.RoomConversationRoot())
	if errors.Is(err, os.ErrNotExist) {
		return []RoomPublicHandoff{}, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result := make([]RoomPublicHandoff, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rows, readErr := s.files.readJSONL(filepath.Join(s.paths.RoomConversationRoot(), entry.Name(), "public_handoffs.jsonl"))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		all := replayRoomPublicHandoffRows(rows)
		for _, value := range all {
			if roomPublicHandoffIsPending(value, now) {
				result = append(result, value)
			}
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].HandoffID < result[j].HandoffID
	})
	return result, nil
}

func roomPublicHandoffIsPending(value RoomPublicHandoff, now int64) bool {
	switch value.Status {
	case roomPublicHandoffActionDetected,
		roomPublicHandoffActionSourceFinished,
		roomPublicHandoffActionQueued:
		return true
	case roomPublicHandoffActionClaimed:
		return now-value.ClaimedAt > roomPublicHandoffClaimTTL.Milliseconds()
	default:
		return false
	}
}

const roomPublicHandoffClaimTTL = 30 * time.Second

func (s *RoomPublicHandoffStore) transition(
	conversationID string,
	handoffID string,
	action string,
	mutate func(*RoomPublicHandoff),
	allowed func(RoomPublicHandoff) bool,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(conversationID)
	if err != nil {
		return err
	}
	value, ok := all[strings.TrimSpace(handoffID)]
	if !ok {
		return nil
	}
	if !allowed(value) {
		return nil
	}
	mutate(&value)
	value.UpdatedAt = time.Now().UnixMilli()
	return s.appendLocked(conversationID, action, value)
}

func (s *RoomPublicHandoffStore) appendLocked(conversationID string, action string, handoff RoomPublicHandoff) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return errors.New("conversation_id is required")
	}
	return s.files.appendJSONL(s.paths.RoomPublicHandoffsPath(conversationID), map[string]any{
		"action":    action,
		"handoff":   handoff,
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *RoomPublicHandoffStore) replayLocked(conversationID string) (map[string]RoomPublicHandoff, error) {
	rows, err := s.files.readJSONL(s.paths.RoomPublicHandoffsPath(strings.TrimSpace(conversationID)))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]RoomPublicHandoff{}, nil
	}
	if err != nil {
		return nil, err
	}
	return replayRoomPublicHandoffRows(rows), nil
}

func replayRoomPublicHandoffRows(rows []map[string]any) map[string]RoomPublicHandoff {
	result := make(map[string]RoomPublicHandoff)
	for _, row := range rows {
		payload, marshalErr := json.Marshal(row["handoff"])
		if marshalErr != nil {
			continue
		}
		var value RoomPublicHandoff
		if json.Unmarshal(payload, &value) != nil || strings.TrimSpace(value.HandoffID) == "" {
			continue
		}
		result[value.HandoffID] = value
	}
	return result
}

func validateRoomPublicHandoff(value RoomPublicHandoff) error {
	for name, field := range map[string]string{
		"handoff_id":        value.HandoffID,
		"conversation_id":   value.ConversationID,
		"source_message_id": value.SourceMessageID,
		"source_agent_id":   value.SourceAgentID,
		"target_agent_id":   value.TargetAgentID,
	} {
		if strings.TrimSpace(field) == "" {
			return errors.New(name + " is required")
		}
	}
	return nil
}

func roomPublicHandoffCanStart(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionSourceFinished, roomPublicHandoffActionQueued:
		return true
	case roomPublicHandoffActionClaimed:
		return time.Now().UnixMilli()-value.ClaimedAt > roomPublicHandoffClaimTTL.Milliseconds()
	default:
		return false
	}
}

func roomPublicHandoffCanTerminal(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionFinished, roomPublicHandoffActionError, roomPublicHandoffActionInterrupted:
		return false
	default:
		return true
	}
}

func roomPublicHandoffCanCancelSource(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionDetected,
		roomPublicHandoffActionSourceFinished,
		roomPublicHandoffActionQueued,
		roomPublicHandoffActionClaimed:
		return true
	default:
		return false
	}
}

func normalizeRoomPublicHandoffTerminalStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "error":
		return roomPublicHandoffActionError
	case "interrupted", "cancelled":
		return roomPublicHandoffActionInterrupted
	default:
		return roomPublicHandoffActionFinished
	}
}

const (
	roomPublicHandoffActionFinished    = "finished"
	roomPublicHandoffActionError       = "error"
	roomPublicHandoffActionInterrupted = "interrupted"
)
