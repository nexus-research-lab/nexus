// INPUT: Room slot 的 runtime result、failure/interruption、WorkBinding 与消息 mapper。
// OUTPUT: 原子持久化的 runtime identity、经最终权限 admission 的 usage/handoff/cursor、可见终态与 structured root Attempt 终态，或静默撤销旧 slot。
// POS: 单 slot 所有终态路径的统一结算边界；不得隐式创建 Submission/Acceptance。
package realtime

import (
	"context"
	"strings"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

func (s *Service) syncSlotRuntimeIdentity(
	ctx context.Context,
	slot *activeRoomSlot,
	sessionID string,
	toolSurfaceFingerprint string,
) (bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, nil
	}
	if !s.canPersistSlotSDKSessionID(ctx, slot, sessionID) {
		return false, nil
	}
	if s.rooms != nil {
		if err := s.rooms.UpdateSessionRuntimeIdentity(
			ctx,
			slot.RoomSessionID,
			sessionID,
			strings.TrimSpace(toolSurfaceFingerprint),
		); err != nil {
			return false, err
		}
	}
	slot.setSDKSessionID(sessionID)
	slot.ensureSDKSessionIdentityState().Set(sessionID)
	return true, nil
}

func (s *Service) canPersistSlotSDKSessionID(ctx context.Context, slot *activeRoomSlot, sessionID string) bool {
	return s.CanPersistSDKSessionID(ctx, slot.OwnerUserID, slotWorkspacePath(slot), sessionID,
		"agent_id", slotAgentID(slot),
		"agent_round_id", slotAgentRoundID(slot),
		"runtime_session_key", slotRuntimeSessionKey(slot),
	)
}

func (s *Service) clearSlotSDKSessionID(ctx context.Context, slot *activeRoomSlot) error {
	if slot == nil {
		return nil
	}
	if s.rooms != nil {
		roomSessionID := strings.TrimSpace(slot.RoomSessionID)
		if roomSessionID != "" {
			if err := s.rooms.UpdateSessionRuntimeIdentity(ctx, roomSessionID, "", ""); err != nil {
				return err
			}
		}
	}
	slot.clearSDKSessionID()
	slot.ensureSDKSessionIdentityState().Set("")
	return nil
}

func slotAgentID(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return strings.TrimSpace(slot.AgentID)
}

func slotAgentRoundID(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return strings.TrimSpace(slot.AgentRoundID)
}

func slotRuntimeSessionKey(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return strings.TrimSpace(slot.RuntimeSessionKey)
}

func slotWorkspacePath(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	return strings.TrimSpace(slot.WorkspacePath)
}

// broadcastAgentRoundStatus 广播 slot 生命周期状态；内部 "cancelled" 对外统一为 "interrupted"。
func (s *Service) broadcastAgentRoundStatus(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, status string) {
	if roundValue == nil || slot == nil {
		return
	}
	if status == "cancelled" {
		status = "interrupted"
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapAgentRoundStatusEvent(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
		slot.AgentID,
		status,
	))
}

func (e *slotExecution) complete(result exec.RoundExecutionResult) error {
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	lastAssistant := e.mapper.LastAssistantMessage()
	if result.CompletedByAssistant {
		e.slot.mutable.goal.RecordTerminalAssistantUsage(lastAssistant, e.writeUsage)
		e.slot.mutable.goal.RememberGoalCompletionAssistant(lastAssistant)
		e.service.persistRoomGoalCompletionReceipt(e.ctx, e.round, e.slot, false)
	}
	e.service.recordGoalUsageLimitForSlot(e.ctx, e.slot, result)
	e.service.recordGoalContinuationProgressForSlot(e.ctx, e.slot, e.round, result, lastAssistant)
	e.service.finalizeGoalUsageForSlot(e.ctx, e.slot, result, lastAssistant)
	terminalStatus := roomSlotTerminalStatus(result)
	if terminalStatus == "error" && strings.TrimSpace(result.ErrorMessage) != "" {
		e.slot.setErrorMessage(result.ErrorMessage)
	}
	if e.slot.getStatus() == "running" {
		e.slot.setStatus(terminalStatus)
	}
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	e.service.broadcastAgentRoundStatus(e.ctx, e.round, e.slot, e.slot.getStatus())
	if err := e.deliverExternalCompletion(result, lastAssistant); err != nil {
		e.logger.Warn("外部会话回信失败，保留 Room 原消息", "err", err)
	}
	if err := e.persistCompletionOutput(lastAssistant); err != nil {
		return err
	}
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	e.service.markPublicHandoffTerminal(e.ctx, e.round, e.slot, e.slot.getStatus())
	if e.slot.getStatus() == "finished" {
		if err := e.commitCompletionCursors(); err != nil {
			return err
		}
	}
	return e.service.finishBoundRoomAttempt(
		e.ctx,
		e.round,
		e.slot,
		e.slot.getStatus(),
		result.ErrorMessage,
	)
}

func (e *slotExecution) persistCompletionOutput(lastAssistant protocol.Message) error {
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	if e.slot.shouldSuppressOutput() {
		return nil
	}
	if err := e.service.recordRoomDirectedMessageReply(e.ctx, e.round, e.slot, lastAssistant); err != nil {
		return err
	}
	if !roomSlotPublishesPublicOutput(e.slot) {
		return nil
	}
	return e.service.collectPublicMentionWakes(e.ctx, e.round, e.slot, lastAssistant)
}

func (e *slotExecution) commitCompletionCursors() error {
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	publicCursorID, publicCursorTS := e.slot.publicCursor()
	if err := e.service.recordRoomPublicCursor(e.slot, e.round, publicCursorID, publicCursorTS); err != nil {
		return err
	}
	messageCursor, recorded, err := e.service.recordRoomDirectedMessageCursor(e.slot, e.round)
	if err != nil || !recorded {
		return err
	}
	e.service.broadcastSharedEventWithTimeout(
		e.ctx,
		e.round.SessionKey,
		e.round.RoomID,
		newRoomDirectedMessageConsumedEvent(messageCursor),
	)
	return nil
}

func (s *Service) handleSlotFailure(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	mapper *roomdomain.SlotMessageMapper,
	result exec.RoundExecutionResult,
	err error,
) {
	if s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, err) {
		return
	}
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	fields := []any{
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"err", err,
	}
	fields = append(fields, roomSlotFailureDiagnostics(err, slot, mapper)...)
	s.LoggerFor(ctx).Error("Room slot 执行失败", fields...)
	displayError := exec.RoundErrorDisplayMessage(err)
	if settleErr := s.finishBoundRoomAttempt(
		ctx,
		roundValue,
		slot,
		"error",
		err.Error(),
	); settleErr != nil {
		s.LoggerFor(ctx).Error(
			"Room structured root Attempt 失败收口失败",
			"dispatch_id",
			executionDispatchID(slot.currentWorkBinding()),
			"err",
			settleErr,
		)
	}
	lastAssistant := slot.mutable.goal.LastGoalAssistantMessage()
	// durable assistant 已进入 slot 内存、但共享/私有历史持久化可能失败。
	// failure 收口仍须用该快照结算并关闭 parent usage，不能只记录错误状态。
	s.finalizeGoalUsageForSlot(ctx, slot, result, lastAssistant)
	s.recordGoalContinuationProgressForSlot(ctx, slot, roundValue, exec.RoundExecutionResult{
		TerminalStatus: "error",
		ErrorMessage:   displayError,
	}, lastAssistant)
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.cancelSourcePublicHandoffs(ctx, roundValue, slot, "error")
	s.markPublicHandoffTerminal(ctx, roundValue, slot, "error")
	slot.setErrorMessage(displayError)
	// 原因先于终态发布，确保 root round 观察到 error 时一定能读取详情。
	slot.setStatus("error")
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.broadcastAgentRoundStatus(ctx, roundValue, slot, "error")
	resultMessage := roomSlotResultMessage(roundValue, slot, "error", displayError, true)
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	_ = s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey))
	if roomSlotPublishesPublicOutput(slot) {
		if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
			s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
			return
		}
		_ = s.persistSharedInlineMessage(roundValue.OwnerUserID, roundValue.ConversationID, resultMessage)
		if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
			s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
			return
		}
		projectedMessage := message.ProjectResultMessage(nil, resultMessage)
		if mapper != nil {
			projectedMessage = mapper.ProjectResultMessage(resultMessage)
		}
		if projectedMessage != nil {
			s.broadcastSharedEventWithTimeout(
				ctx,
				roundValue.SessionKey,
				roundValue.RoomID,
				roomdomain.WrapMessageEvent(
					roundValue.RoomID,
					roundValue.ConversationID,
					projectedMessage,
					roundValue.RootRoundID,
				),
			)
		} else {
			errorEvent := roomdomain.NewErrorEvent(
				roundValue.SessionKey,
				roundValue.RoomID,
				roundValue.ConversationID,
				"room_error",
				displayError,
				roundValue.RootRoundID,
			)
			errorEvent.AgentID = slot.AgentID
			errorEvent.AgentRoundID = slot.AgentRoundID
			s.broadcastSharedEventWithTimeout(
				ctx,
				roundValue.SessionKey,
				roundValue.RoomID,
				errorEvent,
			)
		}
	}
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
	))
}

func roomSlotFailureDiagnostics(err error, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper) []any {
	fields := exec.RoundStreamFailureLogFields(err)
	if mapper != nil {
		lastAssistant := mapper.LastAssistantMessage()
		fields = append(fields,
			"sdk_session_id", mapper.SessionID(),
			"current_message_id", mapper.CurrentMessageID(),
			"last_assistant_message_id", anyString(lastAssistant["message_id"]),
			"last_assistant_complete", lastAssistant["is_complete"],
			"last_assistant_chars", utf8.RuneCountInString(roomdomain.ExtractHistoryText(lastAssistant)),
		)
	}
	if client := slot.getClient(); client != nil {
		fields = append(fields, "client_session_id", client.SessionID())
	}
	return fields
}

func (s *Service) handleSlotCancelled(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	mapper *roomdomain.SlotMessageMapper,
	result exec.RoundExecutionResult,
) {
	if !s.markSlotCancelled(slot) {
		return
	}
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.LoggerFor(ctx).Warn("Room slot 已取消",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"reason", roomSlotInterruptDisplayReason(slot),
	)
	if settleErr := s.finishBoundRoomAttempt(
		ctx,
		roundValue,
		slot,
		"interrupted",
		roomSlotInterruptReason(slot),
	); settleErr != nil {
		s.LoggerFor(ctx).Error(
			"Room structured root Attempt 中断收口失败",
			"dispatch_id",
			executionDispatchID(slot.currentWorkBinding()),
			"err",
			settleErr,
		)
	}
	if mapper != nil {
		s.finalizeGoalUsageForSlot(ctx, slot, result, slot.mutable.goal.LastGoalAssistantMessage())
	}
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.cancelSourcePublicHandoffs(ctx, roundValue, slot, "interrupted")
	s.markPublicHandoffTerminal(ctx, roundValue, slot, "interrupted")
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.emitInterruptedSlotResult(roundValue, slot, mapper, "")
	if authorityErr := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); authorityErr != nil {
		s.retireSlotAfterOutputRevocation(ctx, roundValue, slot, authorityErr)
		return
	}
	s.broadcastSlotCancelled(ctx, roundValue, slot)
}

func (s *Service) markSlotCancelled(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	return slot.markCancelled()
}

func (s *Service) broadcastSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot) {
	s.broadcastAgentRoundStatus(ctx, roundValue, slot, "interrupted")
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamCancelled,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
	))
}

func (s *Service) emitInterruptedSlotResult(roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper, resultText string) {
	if roundValue == nil || slot == nil {
		return
	}
	resultMessage := roomSlotResultMessage(roundValue, slot, "interrupted", resultText, false)
	if client := slot.getClient(); client != nil {
		if sessionID := strings.TrimSpace(client.SessionID()); sessionID != "" {
			resultMessage["session_id"] = sessionID
		}
	}
	if roomSlotPublishesPublicOutput(slot) {
		if err := s.persistSharedInlineMessage(
			roundValue.OwnerUserID,
			roundValue.ConversationID,
			resultMessage,
		); err != nil {
			s.LoggerFor(context.Background()).Error("Room interrupted 共享结果持久化失败",
				"s", roundValue.SessionKey,
				"r", roundValue.RoomID,
				"c", roundValue.ConversationID,
				"err", err,
			)
		} else {
			projectedMessage := message.ProjectResultMessage(nil, resultMessage)
			if mapper != nil {
				projectedMessage = mapper.ProjectResultMessage(resultMessage)
			}
			if projectedMessage != nil {
				s.broadcastSharedEvent(
					context.Background(),
					roundValue.SessionKey,
					roundValue.RoomID,
					roomdomain.WrapMessageEvent(
						roundValue.RoomID,
						roundValue.ConversationID,
						projectedMessage,
						roundValue.RootRoundID,
					),
				)
			}
		}
	}
	if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey)); err != nil {
		s.LoggerFor(context.Background()).Error("Room interrupted 私有结果持久化失败",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"err", err,
		)
	}
}

// roomSlotResultMessage 构造 slot 级宿主 result：message_id 按 Agent 轮次，round_id 是 root round。
func roomSlotResultMessage(roundValue *activeRoomRound, slot *activeRoomSlot, subtype string, text string, isError bool) protocol.Message {
	message := protocol.NewHostResultMessage("result_"+slot.AgentRoundID, roundValue.SessionKey, slot.AgentID, roundValue.RootRoundID, subtype, text, isError)
	message["room_id"] = roundValue.RoomID
	message["conversation_id"] = roundValue.ConversationID
	message["agent_round_id"] = slot.AgentRoundID
	message["parent_id"] = slot.MsgID
	return message
}
