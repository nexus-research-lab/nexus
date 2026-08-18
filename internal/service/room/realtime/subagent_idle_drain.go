// INPUT: 已结束 Room round 的后台子 Agent runtime 消息与捕获的 Room authority epoch。
// OUTPUT: 经即时撤权检查的 durable history、Goal child actual usage、事件投影与 post-round 释放信号。
// POS: Room slot 终态后 idle task 消息与后台输出的最终权限边界。
package realtime

import (
	"context"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (s *Service) startIdleSubagentNotificationDrains(ctx context.Context, roundValue *activeRoomRound) {
	if s == nil || roundValue == nil {
		return
	}
	for _, slot := range roundValue.Slots {
		if slot == nil || !s.runtime.HasSubagentHistory(slot.RuntimeSessionKey) {
			continue
		}
		mapper := roomdomain.NewSlotMessageMapper(
			roundValue.SessionKey,
			roundValue.RoomID,
			roundValue.ConversationID,
			slot.AgentID,
			slot.MsgID,
			roundValue.RootRoundID,
			slot.AgentRoundID,
			slot.WorkspacePath,
		)
		mapper.SetMessageDecorator(func(message protocol.Message) {
			s.decorateRoomMessage(roundValue, slot, message)
		})
		s.runtime.StartIdleMessageDrain(
			slot.RuntimeSessionKey,
			func(drainCtx context.Context, incoming sdkprotocol.ReceivedMessage) bool {
				return s.handleIdleSubagentMessage(
					contextWithExactQueueOwner(drainCtx, roundValue.OwnerUserID),
					roundValue,
					slot,
					mapper,
					incoming,
				)
			},
		)
	}
}

func (s *Service) handleIdleSubagentMessage(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	mapper *roomdomain.SlotMessageMapper,
	incoming sdkprotocol.ReceivedMessage,
) bool {
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, err)
		return false
	}
	s.observeExecutionRuntimeGraph(roomOrchestrationActor(roundValue, slot), incoming)
	events, durableMessages, _, err := mapper.Map(incoming)
	if err != nil {
		s.loggerFor(ctx).Warn("处理 Room idle subagent 通知失败",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
			"agent_id", slot.AgentID,
			"err", err,
		)
		return true
	}
	for _, messageValue := range durableMessages {
		if messageValue == nil {
			continue
		}
		if err := s.handleIdleSubagentDurableMessage(ctx, roundValue, slot, messageValue); err != nil {
			if s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, err) {
				return false
			}
			s.loggerFor(ctx).Warn("写入 Room idle subagent 通知失败",
				"session_key", roundValue.SessionKey,
				"round_id", roundValue.RoundID,
				"agent_id", slot.AgentID,
				"err", err,
			)
			return true
		}
	}
	for _, event := range events {
		if roomSlotShouldDropPublicOutputEvent(slot, event) {
			continue
		}
		if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
			s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, err)
			return false
		}
		for _, readyEvent := range slot.eventsReadyForEmission(event) {
			s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, readyEvent)
		}
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, err)
		return false
	}
	if slot.hasRunningSubagentTask() {
		return true
	}
	if !s.finalizeCompletedRoomGoalUsage(ctx, roundValue) &&
		!roundValue.hasRunningSubagentTasks() {
		s.loggerFor(ctx).Warn(
			"Room child drain 后 Goal usage 尚未完成最终结算",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
		)
		return true
	}
	s.releaseRoundSubagentWait(roundValue)
	// task 完成后仍保留 drain，nxs 可从 UI follow-up 用同一 task ID 再次唤醒。
	return true
}

func (s *Service) handleIdleSubagentDurableMessage(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	messageValue protocol.Message,
) error {
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	settledSubagentUsage := s.recordSubagentGoalUsageForSlot(ctx, slot, messageValue)
	slot.rememberSubagentTaskMessage(messageValue)
	for _, settlement := range settledSubagentUsage {
		slot.clearSubagentUsageObservationPending(settlement.taskID, settlement.observation)
	}
	s.startRoomSubagentUsageRetry(roundValue, slot)
	if slot.hasSubagentHistory() {
		s.runtime.MarkSubagentHistory(slot.RuntimeSessionKey)
	}
	if !roomSlotPublishesPublicOutput(slot) {
		if !protocol.IsTranscriptNativeMessage(messageValue) {
			if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
				return err
			}
			if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
				return err
			}
		}
		if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
			return err
		}
		s.observeExecutionRuntimeArtifacts(roomOrchestrationActor(roundValue, slot), messageValue)
		actor := roomOrchestrationActor(roundValue, slot)
		s.recordGoalUsageFromSlotAssistantMessageWithActor(ctx, slot, &actor, messageValue)
		return nil
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	if err := s.persistSharedDurableMessage(
		roundValue.OwnerUserID,
		roundValue.ConversationID,
		slot,
		messageValue,
	); err != nil {
		return err
	}
	if !protocol.IsTranscriptNativeMessage(messageValue) {
		if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
			return err
		}
		if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
			return err
		}
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	s.observeExecutionRuntimeArtifacts(roomOrchestrationActor(roundValue, slot), messageValue)
	actor := roomOrchestrationActor(roundValue, slot)
	s.recordGoalUsageFromSlotAssistantMessageWithActor(ctx, slot, &actor, messageValue)
	return nil
}

func (s *Service) releaseRoundSubagentWait(roundValue *activeRoomRound) {
	if s == nil || roundValue == nil {
		return
	}
	if !roundValue.hasRunningSubagentTasks() &&
		roundValue.RunningSubagents.CompareAndSwap(true, false) {
		s.startSessionBackgroundTask(
			roundValue.SessionKey,
			roundValue.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchPostRoundWorkOnce(taskCtx, roundValue)
			},
		)
	}
}

// dispatchPostRoundWorkOnce 保证 runRound 收尾与后台 usage worker 竞争时，
// 只有越过最终 settlement barrier 的一方派发一次 post-round。
func (s *Service) dispatchPostRoundWorkOnce(
	ctx context.Context,
	roundValue *activeRoomRound,
) {
	if s == nil || roundValue == nil || roundValue.RunningSubagents.Load() ||
		!roundValue.postRoundDispatched.CompareAndSwap(false, true) {
		return
	}
	s.dispatchPostRoundWork(ctx, roundValue)
}

// startRoomSubagentUsageRetry 为一个 slot 保持至多一个 usage worker。同步写入
// 已经耗尽重试后，worker 从 pending map 读取最新累计值，不依赖新的 runtime 消息。
func (s *Service) startRoomSubagentUsageRetry(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	if s == nil || roundValue == nil || slot == nil ||
		!slot.tryStartSubagentUsageRetry() {
		return
	}
	go s.retryRoomSubagentUsage(roundValue, slot)
}

// startRoomGoalUsageRetry 在 parent terminal settlement 或 shared finalization
// 的同步重试耗尽后复用同一个 slot worker；没有 child pending 也会继续重试。
func (s *Service) startRoomGoalUsageRetry(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	if s == nil || roundValue == nil || slot == nil ||
		!slot.tryStartGoalUsageRetry() {
		return
	}
	go s.retryRoomSubagentUsage(roundValue, slot)
}

func (s *Service) retryRoomSubagentUsage(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	defer s.finishRoomGoalUsageRetryWorker(roundValue, slot)

	ctx := contextWithExactQueueOwner(context.Background(), roundValue.OwnerUserID)
	retryAttempt := 0
	for {
		unlockScope := s.lockRoomGoalUsageScope(ctx, slot)
		pending := slot.subagentUsageObservationPendingSnapshot()
		if len(pending) > 0 {
			goalID := strings.TrimSpace(slot.childGoalIDForUsage())
			goalSessionKey := goalUsageSessionKeyForRoomSlot(slot, goalSessionKeyForSlot(slot))
			for taskID, observation := range pending {
				if _, err := s.persistSubagentGoalUsageObservationForSlot(
					ctx,
					slot,
					taskID,
					observation,
					goalID,
					goalSessionKey,
				); err != nil {
					s.loggerFor(ctx).Warn(
						"后台重试 Room nxs 子任务 Goal usage 失败",
						"session_key", goalSessionKey,
						"goal_id", goalID,
						"scope_round_id", goalUsageScopeRoundIDForRoomSlot(slot),
						"source_round_id", slot.AgentRoundID,
						"task_id", taskID,
						"err", err,
					)
					continue
				}
				slot.clearSubagentUsageObservationPending(taskID, observation)
			}
			if len(slot.subagentUsagePendingSnapshot()) > 0 {
				unlockScope()
				if !s.waitRoomSubagentUsageRetry(retryAttempt) {
					return
				}
				retryAttempt++
				continue
			}
			retryAttempt = 0
		}
		unlockScope()

		// parent 尚未终态时只完成 child checkpoint；正常 runRound 收尾会负责
		// shared finalization 与 post-round，避免 child 提前触发下一轮。
		if !roomRoundParentSlotsTerminal(roundValue) {
			return
		}
		if s.finalizeCompletedRoomGoalUsage(ctx, roundValue) {
			s.releaseRoundSubagentWait(roundValue)
			return
		}
		if !s.waitRoomSubagentUsageRetry(retryAttempt) {
			return
		}
		retryAttempt++
	}
}

func (s *Service) finishRoomGoalUsageRetryWorker(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	if slot.finishSubagentUsageRetry() {
		s.startRoomSubagentUsageRetry(roundValue, slot)
		return
	}
	// runRound 可能恰好在旧 worker 成功退出后重新建立 parent
	// settlement barrier；清 worker flag 后重新检查并主动接棒，避免永久 pending。
	if roundValue.RunningSubagents.Load() &&
		!roundValue.postRoundDispatched.Load() &&
		roomRoundParentSlotsTerminal(roundValue) {
		s.startRoomGoalUsageRetry(roundValue, slot)
	}
}

func roomRoundParentSlotsTerminal(roundValue *activeRoomRound) bool {
	if roundValue == nil || len(roundValue.Slots) == 0 {
		return false
	}
	for _, slot := range roundValue.Slots {
		if slot != nil && !slot.isTerminal() {
			return false
		}
	}
	return true
}

func (s *Service) waitRoomSubagentUsageRetry(attempt int) bool {
	baseDelay := 20 * time.Millisecond
	if s != nil && s.goalUsageRetryBaseDelay > 0 {
		baseDelay = s.goalUsageRetryBaseDelay
	}
	delay := baseDelay * time.Duration(1<<min(attempt, 8))
	if delay > roomGoalUsageRetryMaxDelay {
		delay = roomGoalUsageRetryMaxDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
	return true
}
