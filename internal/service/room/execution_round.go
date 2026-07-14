// INPUT: 已构造的 Room round、历史与 Agent 目录。
// OUTPUT: slot 终态、共享事件，以及用户队列优先的后续工作接力。
// POS: Room round 生命周期的唯一收尾编排入口。
package room

import (
	"context"
	"sync"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
	ctx = contextWithQueueOwner(ctx, roundValue.OwnerUserID)
	logger := s.loggerFor(ctx).With(
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
	)
	logger.Info("开始执行 Room round", "slot_count", len(roundValue.Slots))
	var waitGroup sync.WaitGroup
	for _, slot := range roundValue.Slots {
		waitGroup.Add(1)
		go func(currentSlot *activeRoomSlot) {
			defer waitGroup.Done()
			s.runSlot(ctx, roundValue, currentSlot, history, agentNameByID, agentByID[currentSlot.AgentID])
			// 每个 Agent 独立串行。当前 slot 已终态且 runtime 清理完成后，
			// 立即释放它错过的 guide 并派发其队列，不等待同 root 的其他成员。
			dispatchCtx := contextWithQueueOwner(context.Background(), roundValue.OwnerUserID)
			s.releaseUndeliveredRoomGuidance(dispatchCtx, roundValue.SessionKey, roundValue.Context)
			s.dispatchNextInputQueueItem(dispatchCtx, roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID)
		}(slot)
	}
	waitGroup.Wait()

	roundValue.RunningSubagents.Store(roundValue.hasRunningSubagentTasks())
	// Interrupt 只等待执行体结束；queue/guide 交接仍在下方锁内收口。
	roundValue.doneOnce.Do(func() { close(roundValue.Done) })
	s.inputQueueDispatchMu.Lock()
	s.finishRound(roundValue)
	s.inputQueueDispatchMu.Unlock()

	finalStatus := "finished"
	if roundValue.allSlotsCancelled() {
		finalStatus = "interrupted"
	} else if roundValue.hasSlotError() {
		finalStatus = "error"
	}
	logger.Info("Room round 结束", "status", finalStatus)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapRoundStatusEvent(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		roundValue.RoundID,
		finalStatus,
		mapTerminalSubtype(finalStatus),
	))
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	// 显式用户输入先于 Agent 唤醒和 Goal 隐藏续跑；错过 hook 的 guide 自动退回下一轮。
	s.releaseUndeliveredRoomGuidance(ctx, roundValue.SessionKey, roundValue.Context)
	s.dispatchNextInputQueueItem(ctx, roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID)
	// 只要 slot runtime 留有 subagent history 就继续接管消息；终态 task 也可能被 UI follow-up 唤醒。
	s.startIdleSubagentNotificationDrains(contextWithQueueOwner(context.Background(), roundValue.OwnerUserID), roundValue)
	if finalStatus == "finished" {
		s.startQueuedPublicMentionWakes(context.Background(), roundValue)
	}
	go s.dispatchPostRoundWork(contextWithQueueOwner(context.Background(), roundValue.OwnerUserID), roundValue)
}
