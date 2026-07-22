package realtime

import (
	"context"
	"sort"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// ActiveRoundSnapshot 表示 Room 当前仍在执行的主轮次快照。
type ActiveRoundSnapshot struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	RoundID        string
	Pending        []protocol.ChatAckPendingSlot
}

// CountRunningTasks 返回指定 Agent 当前在 Room 中的活跃任务数。
func (s *Service) CountRunningTasks(agentID string) int {
	count := 0
	for _, roundValue := range s.rounds.snapshot() {
		for _, slot := range roundValue.Slots {
			if slot != nil && slot.AgentID == agentID && !slot.isTerminal() {
				count++
			}
		}
	}
	return count
}

// SetPermissionModeForAgent 将权限模式热同步到指定 agent 已存在的 Room runtime。
func (s *Service) SetPermissionModeForAgent(ctx context.Context, agentID string, mode sdkpermission.Mode) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil
	}
	clients := make([]runtimectx.Client, 0)
	for _, roundValue := range s.rounds.snapshot() {
		if roundValue == nil {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.AgentID != agentID || slot.isTerminal() {
				continue
			}
			client := slot.getClient()
			if client == nil {
				continue
			}
			clients = append(clients, client)
		}
	}
	for _, client := range clients {
		if err := client.SetPermissionMode(ctx, mode); err != nil {
			return err
		}
	}
	return nil
}

// GetActiveRoundSnapshot 返回指定 conversation 的活跃 slot 快照。
func (s *Service) GetActiveRoundSnapshot(conversationID string) *ActiveRoundSnapshot {
	pending := make([]protocol.ChatAckPendingSlot, 0)
	snapshot := &ActiveRoundSnapshot{}
	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil || roundValue.ConversationID != conversationID {
			continue
		}
		if snapshot.SessionKey == "" {
			snapshot.SessionKey = roundValue.SessionKey
			snapshot.RoomID = roundValue.RoomID
			snapshot.ConversationID = roundValue.ConversationID
			snapshot.RoundID = roomRootRoundID(roundValue)
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || slot.isTerminal() {
				continue
			}
			status := slot.getStatus()
			if status == "running" {
				status = "streaming"
			}
			pending = append(pending, protocol.ChatAckPendingSlot{
				AgentID:      slot.AgentID,
				AgentRoundID: slot.AgentRoundID,
				MsgID:        slot.MsgID,
				Status:       status,
				Timestamp:    slot.TimestampMS,
				Index:        slot.Index,
			})
		}
	}
	if len(pending) == 0 {
		return nil
	}
	sort.Slice(pending, func(i int, j int) bool {
		if pending[i].Timestamp != pending[j].Timestamp {
			return pending[i].Timestamp < pending[j].Timestamp
		}
		return pending[i].Index < pending[j].Index
	})
	snapshot.Pending = pending
	return snapshot
}

func (s *Service) registerRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.rounds.register(roundValue)
}

func (s *Service) finishRound(roundValue *activeRoomRound) {
	if roundValue == nil {
		return
	}
	s.runtime.MarkRoundFinished(roundValue.SessionKey, roundValue.RoundID)
	s.rounds.unregister(roundValue)
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

// firstSlotErrorMessage 返回按展示顺序最早出现的 slot 错误。
func (r *activeRoomRound) firstSlotErrorMessage() string {
	if r == nil {
		return ""
	}
	var firstMessage string
	firstIndex := 0
	found := false
	for _, slot := range r.Slots {
		if slot == nil {
			continue
		}
		message := slot.getErrorMessage()
		if message == "" || (found && slot.Index >= firstIndex) {
			continue
		}
		firstIndex = slot.Index
		firstMessage = message
		found = true
	}
	return firstMessage
}

func (r *activeRoomRound) hasRunningSubagentTasks() bool {
	if r == nil {
		return false
	}
	for _, slot := range r.Slots {
		if slot != nil && slot.hasRunningSubagentTask() {
			return true
		}
	}
	return false
}
