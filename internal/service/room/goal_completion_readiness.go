// INPUT: 当前 Room Goal、调用方 Agent/root round、active slots 与 durable Room work。
// OUTPUT: complete 前第一个 outstanding-work blocker；调用方主 slot 不阻塞自身。
// POS: Room 实时/持久化工作到 Goal 终态 gate 的唯一投影入口。
package room

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RoomGoalCompletionBlocker 返回阻止共享 Goal complete 的 Room 工作；空字符串表示已收敛。
func (s *RealtimeService) RoomGoalCompletionBlocker(
	ctx context.Context,
	goal protocol.Goal,
	callerAgentID string,
	callerRoundID string,
) (string, error) {
	if s == nil || !protocol.IsRoomSharedSessionKey(goal.SessionKey) {
		return "", nil
	}
	parsed := protocol.ParseSessionKey(goal.SessionKey)
	conversationID := strings.TrimSpace(parsed.ConversationID)
	if conversationID == "" {
		return "", nil
	}

	// input -> public mention 是 Room 现有派发路径的锁顺序。在同一快照内
	// 观测 pending wake、active slot 和 durable queue，避免 wake 交接窗口被误判为 idle。
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()
	s.publicMentionDispatchMu.Lock()
	defer s.publicMentionDispatchMu.Unlock()

	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, true)
	if err != nil {
		return "", err
	}
	if blocker := s.activeRoomGoalBlocker(goal.SessionKey, conversationID, callerAgentID, callerRoundID); blocker != "" {
		return blocker, nil
	}
	if blocker, err := s.roomGoalInputQueueBlocker(ctx, contextValue); err != nil || blocker != "" {
		return blocker, err
	}
	return s.roomGoalDelayedWakeBlocker(conversationID)
}

func (s *RealtimeService) activeRoomGoalBlocker(
	sessionKey string,
	conversationID string,
	callerAgentID string,
	callerRoundID string,
) string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)
	callerAgentID = strings.TrimSpace(callerAgentID)
	callerRoundID = strings.TrimSpace(callerRoundID)

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, roundValue := range s.activeRounds {
		if roundValue == nil ||
			strings.TrimSpace(roundValue.SessionKey) != sessionKey ||
			strings.TrimSpace(roundValue.ConversationID) != conversationID {
			continue
		}
		// public @ 已从模型输出解析，但尚未交接成目标 slot。
		// 它挂在当前 shared Goal 的 Room round 上，清空或注册 slot 后自动解锁。
		if len(roundValue.PublicMentions) > 0 {
			return "a Room public-mention wake has not started"
		}
		for _, slot := range roundValue.Slots {
			if slot == nil {
				continue
			}
			isCallerSlot := callerAgentID != "" && callerRoundID != "" &&
				strings.TrimSpace(slot.AgentID) == callerAgentID &&
				(roomRootRoundID(roundValue) == callerRoundID ||
					strings.TrimSpace(roundValue.RoundID) == callerRoundID ||
					strings.TrimSpace(slot.AgentRoundID) == callerRoundID)
			if slot.hasRunningSubagentTask() {
				if isCallerSlot {
					return fmt.Sprintf("caller agent %s still has running subagent work", callerAgentID)
				}
				return fmt.Sprintf("agent %s still has running subagent work", strings.TrimSpace(slot.AgentID))
			}
			if slot.isTerminal() {
				continue
			}
			if isCallerSlot {
				continue
			}
			return fmt.Sprintf("agent %s still has an active Room slot", strings.TrimSpace(slot.AgentID))
		}
	}
	return ""
}

func (s *RealtimeService) roomGoalInputQueueBlocker(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (string, error) {
	if s.inputQueue == nil || contextValue == nil {
		return "", nil
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	// InputQueue replay 已排除 expired/deleted/dispatched 项。队列尚无 goal_id，
	// 所以对同 conversation 的 active shared Goal 保守阻止，不会被日志历史永久卡住。
	return fmt.Sprintf("Room input queue item %s has not been consumed", strings.TrimSpace(entries[0].Item.ID)), nil
}

func (s *RealtimeService) roomGoalDelayedWakeBlocker(conversationID string) (string, error) {
	if s.directedWakes == nil {
		return "", nil
	}
	pending, err := s.directedWakes.Pending()
	if err != nil {
		return "", err
	}
	for _, wake := range pending {
		if strings.TrimSpace(wake.Message.ConversationID) != strings.TrimSpace(conversationID) {
			continue
		}
		return fmt.Sprintf("Room directed wake %s has not started", strings.TrimSpace(wake.WakeID)), nil
	}
	return "", nil
}
