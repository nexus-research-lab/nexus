package room

import (
	"sort"
	"strings"
)

// ActiveRoundSnapshot 表示 Room 当前仍在执行的主轮次快照。
type ActiveRoundSnapshot struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoundID        string
	Pending        []map[string]any
}

// CountRunningTasks 返回指定 Agent 当前在 Room 中的活跃任务数。
func (s *RealtimeService) CountRunningTasks(agentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, roundValue := range s.activeRounds {
		for _, slot := range roundValue.Slots {
			if slot != nil && slot.AgentID == agentID && !slot.isTerminal() {
				count++
			}
		}
	}
	return count
}

// GetActiveRoundSnapshot 返回指定 conversation 的活跃 slot 快照。
func (s *RealtimeService) GetActiveRoundSnapshot(conversationID string) *ActiveRoundSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	pending := make([]map[string]any, 0)
	snapshot := &ActiveRoundSnapshot{}
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || roundValue.ConversationID != conversationID {
			continue
		}
		if snapshot.SessionKey == "" {
			snapshot.SessionKey = roundValue.SessionKey
			snapshot.RoomID = roundValue.RoomID
			snapshot.ConversationID = roundValue.ConversationID
			snapshot.RoundID = roundValue.RoundID
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.isTerminal() {
				continue
			}
			status := slot.getStatus()
			if status == "running" {
				status = "streaming"
			}
			pending = append(pending, map[string]any{
				"agent_id":  slot.AgentID,
				"msg_id":    slot.MsgID,
				"round_id":  slot.AgentRoundID,
				"status":    status,
				"timestamp": slot.TimestampMS,
				"index":     slot.Index,
			})
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.Slice(pending, func(i int, j int) bool {
		leftTime := normalizeInt64(pending[i]["timestamp"])
		rightTime := normalizeInt64(pending[j]["timestamp"])
		if leftTime != rightTime {
			return leftTime < rightTime
		}
		return intValue(pending[i]["index"]) < intValue(pending[j]["index"])
	})
	for _, item := range pending {
		delete(item, "index")
	}
	snapshot.Pending = pending
	return snapshot
}

func (s *RealtimeService) registerRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.mu.Lock()
	s.activeRounds[roomActiveRoundKey(roundValue.SessionKey, roundValue.RoundID)] = roundValue
	s.mu.Unlock()
}

func (s *RealtimeService) finishRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.runtime.MarkRoundFinished(roundValue.SessionKey, roundValue.RoundID)
	s.mu.Lock()
	delete(s.activeRounds, roomActiveRoundKey(roundValue.SessionKey, roundValue.RoundID))
	s.mu.Unlock()
	roundValue.doneOnce.Do(func() {
		close(roundValue.Done)
	})
}

func roomRootRoundID(roundValue *activeRoomRound) string {
	if roundValue == nil {
		return ""
	}
	if rootRoundID := strings.TrimSpace(roundValue.RootRoundID); rootRoundID != "" {
		return rootRoundID
	}
	return strings.TrimSpace(roundValue.RoundID)
}

func roomActiveRoundKey(sessionKey string, roundID string) string {
	return strings.TrimSpace(sessionKey) + "::" + strings.TrimSpace(roundID)
}

func (r *activeRoomRound) allSlotsCancelled() bool {
	if len(r.Slots) == 0 {
		return false
	}
	for _, slot := range r.Slots {
		if slot == nil || slot.getStatus() != "cancelled" {
			return false
		}
	}
	return true
}

func (r *activeRoomRound) hasSlotError() bool {
	if r == nil {
		return false
	}
	for _, slot := range r.Slots {
		if slot != nil && slot.getStatus() == "error" {
			return true
		}
	}
	return false
}
