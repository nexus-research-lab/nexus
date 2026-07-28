// INPUT: Room slot、运行时消息流、实时插话确认与 Goal 执行上下文。
// OUTPUT: 单个 Room Agent round 的 ACK 门控事件、持久化快照、用量与终态。
// POS: Room 实时编排中把 runtime 输出投影为产品语义的执行主链。

package realtime

import (
	"cmp"
	"context"
	"errors"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

func appendPromptSection(base string, section string) string {
	base = strings.TrimSpace(base)
	section = strings.TrimSpace(section)
	switch {
	case base == "":
		return section
	case section == "":
		return base
	default:
		return base + "\n\n---\n\n" + section
	}
}

// slotExecution 收拢单个 Room slot 的执行态，避免业务阶段之间传递成组参数。
type slotExecution struct {
	service       *Service
	ctx           context.Context
	round         *activeRoomRound
	slot          *activeRoomSlot
	history       []protocol.Message
	agentNameByID map[string]string
	agent         *protocol.Agent
	logger        *slog.Logger
	streamLogger  *slog.Logger
	mapper        *roomdomain.SlotMessageMapper
}

type roomRoundMapperAdapter struct {
	mapper *roomdomain.SlotMessageMapper
}

func (a roomRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (exec.RoundMapResult, error) {
	result, err := a.mapper.MapResult(incoming, interruptReason...)
	if err != nil {
		return exec.RoundMapResult{}, err
	}
	return exec.RoundMapResult{
		Events:          result.Events,
		DurableMessages: result.DurableMessages,
		TerminalStatus:  result.TerminalStatus,
		ResultSubtype:   result.ResultSubtype,
	}, nil
}

func (a roomRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}

func (s *Service) recordUsage(roundValue *activeRoomRound, slot *activeRoomSlot, message protocol.Message) {
	if s.usage == nil || roundValue == nil || slot == nil || protocol.MessageRole(message) != "result" {
		return
	}
	if !usagesvc.MessageHasUsage(message) {
		return
	}
	if s.writeUsage(roundValue, message) {
		slot.setResultUsageWritten()
	}
}

func (s *Service) recordTerminalAssistantUsage(roundValue *activeRoomRound, slot *activeRoomSlot, message protocol.Message) {
	if s.usage == nil || roundValue == nil || slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	if slot.resultUsageWasWritten() || !usagesvc.MessageHasUsage(message) {
		return
	}
	s.writeUsage(roundValue, message)
}

func (s *Service) writeUsage(roundValue *activeRoomRound, message protocol.Message) bool {
	input := usagesvc.MessageRecordInput(roundValue.OwnerUserID, "room_runtime", message)
	if err := s.usage.RecordMessageUsage(context.Background(), input); err != nil {
		s.loggerFor(context.Background()).Error("Room token usage 写入失败",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"err", err,
		)
		return false
	}
	return true
}

func (s *Service) runSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentValue *protocol.Agent,
) {
	if agentValue == nil {
		slot.setErrorMessage("Room slot 缺少 agent 配置")
		slot.setStatus("error")
		s.loggerFor(ctx).Error("Room slot 缺少 agent 配置",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
		)
		return
	}

	slotCtx, cancel := context.WithCancel(ctx)
	slot.setCancel(cancel)
	logger := s.loggerFor(slotCtx).With(
		"s", roundValue.SessionKey,
		"r", roundValue.RoomID,
		"c", roundValue.ConversationID,
	)
	streamLogger := s.loggerFor(slotCtx).With(
		"s", roundValue.SessionKey,
		"a", slot.AgentID,
	)
	mapper := roomdomain.NewSlotMessageMapper(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
		agentValue.WorkspacePath,
	)
	mapper.SetDurableMessageTransformer(func(message protocol.Message) protocol.Message {
		return s.transformRoomDurableMessage(roundValue, slot, message)
	})
	mapper.SetProjectedMessageTransformer(func(message protocol.Message) protocol.Message {
		return s.transformRoomDurableMessage(roundValue, slot, message)
	})
	execution := &slotExecution{
		service:       s,
		ctx:           slotCtx,
		round:         roundValue,
		slot:          slot,
		history:       history,
		agentNameByID: agentNameByID,
		agent:         agentValue,
		logger:        logger,
		streamLogger:  streamLogger,
		mapper:        mapper,
	}
	slot.setStatus("running")
	s.broadcastAgentRoundStatus(slotCtx, roundValue, slot, "running")
	logger.Info("开始执行 Room slot")
	defer s.finishSlot(slot)

	s.permission.BindSessionRoute(slot.RuntimeSessionKey, permissionctx.RouteContext{
		DispatchSessionKey: roundValue.SessionKey,
		RoomID:             roundValue.RoomID,
		ConversationID:     roundValue.ConversationID,
		AgentID:            slot.AgentID,
		MessageID:          slot.MsgID,
		RoundID:            roundValue.RootRoundID,
		AgentRoundID:       slot.AgentRoundID,
	})
	defer s.permission.UnbindSessionRoute(slot.RuntimeSessionKey)

	client, err := execution.prepareRuntimeClient()
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	if !s.runtime.StartRound(slot.RuntimeSessionKey, slot.AgentRoundID, cancel) {
		s.handleSlotFailure(
			slotCtx,
			roundValue,
			slot,
			mapper,
			runtimectx.ErrRuntimeSessionClosing,
		)
		return
	}
	defer func() {
		s.runtime.MarkRoundFinished(slot.RuntimeSessionKey, slot.AgentRoundID)
	}()
	cleanupGoalRuntime := s.registerSlotGoalRuntime(slot)
	defer cleanupGoalRuntime()

	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamStart,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
	))

	result, err := execution.executeRound(client)
	if err != nil {
		if errors.Is(err, exec.ErrRoundInterrupted) {
			s.handleSlotCancelled(slotCtx, roundValue, slot, mapper)
			return
		}
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	if s.shouldConfirmRoomGuidanceByFallback(slot) &&
		result.TerminalStatus == "finished" &&
		(result.ResultSubtype == "" || result.ResultSubtype == "success") {
		if ackErr := s.acknowledgeRoomSlotGuidance(slotCtx, roundValue, slot, nil); ackErr != nil {
			logger.Warn("确认 Room 引导消费失败，保留为后续队列输入", "err", ackErr)
		}
	}

	if err := execution.complete(result); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
	))
	logger.Info("Room slot 结束",
		"status", slot.getStatus(),
		"result_subtype", strings.TrimSpace(result.ResultSubtype),
		"error_message", strings.TrimSpace(result.ErrorMessage),
	)
}

func (e *slotExecution) executeRound(client runtimectx.Client) (exec.RoundExecutionResult, error) {
	payload, err := e.prepareDispatchPayload()
	if err != nil {
		return exec.RoundExecutionResult{}, err
	}
	e.slot.beginNoReplyCandidate()
	return exec.ExecuteRound(e.ctx, exec.RoundExecutionRequest{
		Content:          payload,
		ContextualInputs: goalContextualInputs(e.slot.goalContext(), e.slot.goalIDForUsage(), goalSessionKeyForSlot(e.slot)),
		InputOptions:     runtimectx.RuntimeInputOptionsForPurpose(roomRoundInputOptions(e.round), "goal_continuation"),
		Client:           client,
		Mapper:           roomRoundMapperAdapter{mapper: e.mapper},
		IdleTimeout:      e.service.config.RuntimeRoundIdleTimeout(),
		InterruptReason: func() string {
			return roomSlotInterruptReason(e.slot)
		},
		AfterQuery: func() error {
			return e.sendQueuedInputs(client)
		},
		ObserveIncomingMessage: e.observeIncomingMessage,
		SyncSessionID: func(sessionID string) error {
			return e.service.syncSlotSDKSessionID(e.ctx, e.slot, sessionID)
		},
		HandleDurableMessage: e.handleDurableMessage,
		EmitEvent:            e.emitEvent,
	})
}

func (e *slotExecution) prepareDispatchPayload() (any, error) {
	dispatchPrompt, err := e.service.buildSlotVisibleContext(e.ctx, e.round, e.slot, e.history, e.agentNameByID)
	if err != nil {
		return nil, err
	}
	if err = e.service.recordPrivateRoundMarker(e.round, e.slot, dispatchPrompt); err != nil {
		return nil, err
	}
	runtimeContent, err := e.service.renderRuntimeContentWithAttachments(e.ctx, dispatchPrompt, e.slot.TriggerAttachments)
	if err != nil {
		return nil, err
	}
	runtimeContent = e.service.appendRuntimeUserContext(e.ctx, e.round.ConversationID, e.agent, runtimeContent)
	return runtimeContent.Payload(), nil
}

func (e *slotExecution) sendQueuedInputs(client runtimectx.Client) error {
	for _, input := range e.slot.drainQueuedInputs() {
		if err := runtimectx.SendClientContent(e.ctx, client, input.Content); err != nil {
			return err
		}
		e.logger.Info("发送已排队的 Room 消息",
			"queued_round_id", input.RoundID,
			"content_chars", utf8.RuneCountInString(input.Content),
			"content_preview", logx.PreviewText(input.Content, 240),
		)
	}
	return nil
}

func (e *slotExecution) observeIncomingMessage(incoming sdkprotocol.ReceivedMessage) {
	if !e.streamLogger.Enabled(e.ctx, slog.LevelDebug) {
		return
	}
	if incoming.Type == sdkprotocol.MessageTypeStreamEvent && !e.service.config.MessageDebugStreamEvent {
		return
	}
	fields := trace.BuildSDKMessageLogFieldsWithOptions(
		incoming,
		trace.SDKMessageLogOptions{
			IncludeStreamEvent:  e.service.config.MessageDebugStreamEvent,
			IncludeSnapshotData: true,
		},
	)
	if len(fields) == 0 {
		return
	}
	e.streamLogger.Debug("Room slot 收到 SDK 消息", fields...)
}

func (e *slotExecution) handleDurableMessage(messageValue protocol.Message) error {
	messageRole := protocol.MessageRole(messageValue)
	resultSubtype, _ := messageValue["subtype"].(string)
	resultSubtype = strings.TrimSpace(resultSubtype)
	if e.service.shouldConfirmRoomGuidanceByFallback(e.slot) &&
		(messageRole == "assistant" || (messageRole == "result" && messageValue["is_error"] != true &&
			(resultSubtype == "" || resultSubtype == "success"))) {
		if err := e.service.acknowledgeRoomSlotGuidance(e.ctx, e.round, e.slot, nil); err != nil {
			return err
		}
	}
	e.slot.rememberSubagentTaskMessage(messageValue)
	if e.slot.hasSubagentHistory() {
		e.service.runtime.MarkSubagentHistory(e.slot.RuntimeSessionKey)
	}
	if messageRole == "result" {
		e.slot.setStatus(resultStatus(messageValue["subtype"]))
		e.service.recordUsage(e.round, e.slot, messageValue)
	}
	if messageRole == "assistant" {
		e.slot.rememberGoalAssistantMessage(messageValue)
	}
	if roomdomain.IsNoReplyOutputMessage(messageValue) {
		e.slot.suppressOutput()
		return nil
	}
	if e.slot.shouldSuppressOutput() {
		return nil
	}

	// 无回复标记只控制当前投递，不属于可持久化的对话正文。
	messageValue = roomdomain.StripNoReplyMarker(messageValue)
	// fanout 只是 handoff 路由控制，不应泄漏到 transcript、私域 overlay 或公区。
	messageValue = roomdomain.StripFanoutMarker(messageValue)
	if roomSlotPublishesPublicOutput(e.slot) {
		if err := e.service.persistSharedDurableMessage(
			e.round.OwnerUserID,
			e.round.ConversationID,
			e.slot,
			messageValue,
		); err != nil {
			return err
		}
	}
	if !protocol.IsTranscriptNativeMessage(messageValue) {
		if err := e.service.persistPrivateOverlayMessage(e.slot, cloneMessageWithSessionKey(messageValue, e.slot.RuntimeSessionKey)); err != nil {
			return err
		}
	}
	e.service.recordGoalUsageFromSlotAssistantMessage(e.ctx, e.slot, messageValue)
	return nil
}

func (e *slotExecution) emitEvent(event protocol.EventMessage) error {
	if roomSlotShouldDropPublicOutputEvent(e.slot, event) {
		return nil
	}
	for _, readyEvent := range e.slot.eventsReadyForEmission(event) {
		e.service.broadcastSharedEventWithTimeout(e.ctx, e.round.SessionKey, e.round.RoomID, readyEvent)
	}
	return nil
}

// INPUT: 已构造的 Room round、历史与 Agent 目录。
// OUTPUT: slot 终态、共享事件，以及用户队列优先的后续工作接力。
// POS: Room round 生命周期的唯一收尾编排入口。
func (s *Service) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
	defer s.runtime.MarkRoundFinished(roundValue.SessionKey, roundValue.RoundID)
	ctx = contextWithExactQueueOwner(ctx, roundValue.OwnerUserID)
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
			dispatchCtx := contextWithExactQueueOwner(context.Background(), roundValue.OwnerUserID)
			s.releaseUndeliveredRoomGuidance(dispatchCtx, roundValue.SessionKey, roundValue.Context)
			s.dispatchNextInputQueueItem(dispatchCtx, roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID)
		}(slot)
	}
	waitGroup.Wait()

	roundValue.RunningSubagents.Store(roundValue.hasRunningSubagentTasks())
	// Interrupt 只等待执行体结束；queue/guide 交接仍在下方锁内收口。
	roundValue.doneOnce.Do(func() { close(roundValue.Done) })
	func() {
		lease := s.lockRoomDispatch(roundValue.SessionKey, roundValue.ConversationID)
		defer lease.Unlock()
		s.finishRound(roundValue)
	}()

	finalStatus := "finished"
	if roundValue.allSlotsCancelled() {
		finalStatus = "interrupted"
	} else if roundValue.hasSlotError() {
		finalStatus = "error"
	}
	logger.Info("Room round 结束", "status", finalStatus)
	statusEvent := roomdomain.WrapRoundStatusEvent(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		roundValue.RoundID,
		finalStatus,
		mapTerminalSubtype(finalStatus),
	)
	if finalStatus == "error" {
		statusEvent = roomdomain.WrapRoundStatusErrorEvent(
			roundValue.SessionKey,
			roundValue.RoomID,
			roundValue.ConversationID,
			roundValue.RoundID,
			roundValue.firstSlotErrorMessage(),
		)
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, statusEvent)
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	// Round 已经结束后，所有仍可能写 queue/workspace 或启动后续 runtime 的工作
	// 必须先登记到 session 生命周期，再执行。否则 CloseSession 可能在
	// round 进入终态与这些写盘操作之间返回，迟到 goroutine 会重新创建已清理目录。
	s.startSessionBackgroundTask(
		roundValue.SessionKey,
		roundValue.OwnerUserID,
		func(taskCtx context.Context) {
			// 显式用户输入先于 Agent 唤醒和 Goal 隐藏续跑；错过 hook 的
			// guide 自动退回下一轮。
			s.releaseUndeliveredRoomGuidance(taskCtx, roundValue.SessionKey, roundValue.Context)
			s.dispatchNextInputQueueItem(taskCtx, roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID)
			// 只要 slot runtime 留有 subagent history 就继续接管消息；
			// 终态 task 也可能被 UI follow-up 唤醒。
			s.startIdleSubagentNotificationDrains(taskCtx, roundValue)
			if finalStatus == "finished" {
				s.startQueuedPublicMentionWakes(taskCtx, roundValue)
			}
			s.dispatchPostRoundWork(taskCtx, roundValue)
		},
	)
}

func (s *Service) recordPrivateRoundMarker(roundValue *activeRoomRound, slot *activeRoomSlot, dispatchPrompt string) error {
	if s.history == nil {
		return nil
	}
	options := roomRoundMarkerOptions(roundValue)
	// 私有会话内 slot 自成一轮，round 与 agent round 同源。
	options.AgentRoundID = slot.AgentRoundID
	return s.history.ForOwner(roundValue.OwnerUserID).AppendRoundMarkerWithOptions(
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		slot.AgentRoundID,
		strings.TrimSpace(dispatchPrompt),
		time.Now().UnixMilli(),
		options,
	)
}

func roomRoundInputOptions(roundValue *activeRoomRound) sdkprotocol.OutboundMessageOptions {
	if roundValue == nil {
		return sdkprotocol.OutboundMessageOptions{}
	}
	options := roundValue.InputOptions
	if roundValue.Internal {
		options.HiddenFromUser = true
		options.Synthetic = true
		if strings.TrimSpace(options.Priority) == "" {
			options.Priority = "internal"
		}
	}
	return options
}

func roomRoundMarkerOptions(roundValue *activeRoomRound) workspacestore.RoundMarkerOptions {
	options := workspacestore.RoundMarkerOptions{}
	if roundValue == nil {
		return options
	}
	options.HiddenFromUser = roundValue.Internal || roundValue.InputOptions.HiddenFromUser
	options.Synthetic = roundValue.InputOptions.Synthetic
	options.Purpose = roundValue.InputOptions.Purpose
	options.Metadata = roundValue.InputOptions.Metadata
	if roundValue.Internal {
		options.Synthetic = true
	}
	return options
}

func (s *Service) persistPrivateOverlayMessage(slot *activeRoomSlot, message protocol.Message) error {
	if s.history == nil {
		return nil
	}
	privateMessage := normalizePrivateOverlayMessage(cloneMessageWithSessionKey(message, slot.RuntimeSessionKey))
	privateMessage["session_key"] = slot.RuntimeSessionKey
	// 私有会话内 slot 自成一轮：round 对齐私有 round marker（= agent_round_id），
	// 避免与共享历史的 root round 混用导致私有轮被拆开。
	if agentRoundID := strings.TrimSpace(slot.AgentRoundID); agentRoundID != "" {
		privateMessage["round_id"] = agentRoundID
		privateMessage["agent_round_id"] = agentRoundID
	}
	if sessionID := cmp.Or(strings.TrimSpace(anyString(privateMessage["session_id"])), slot.getSDKSessionID()); sessionID != "" {
		privateMessage["session_id"] = sessionID
	}
	if strings.TrimSpace(anyString(privateMessage["message_id"])) == "" {
		privateMessage["message_id"] = "overlay_" + slot.AgentRoundID
	}
	privateMessage["metadata"] = mergePrivateOverlayMetadata(privateMessage["metadata"], map[string]any{
		"overlay_source":  "room_runtime",
		"room_session_id": slot.RoomSessionID,
	})
	return s.history.ForOwner(slot.OwnerUserID).AppendOverlayMessage(
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		privateMessage,
	)
}

func normalizePrivateOverlayMessage(message protocol.Message) protocol.Message {
	normalized := cloneMessageWithSessionKey(message, anyString(message["session_key"]))
	delete(normalized, "stream_status")
	delete(normalized, "is_complete")
	return normalized
}

func mergePrivateOverlayMetadata(current any, extra map[string]any) map[string]any {
	result := map[string]any{}
	if payload, ok := current.(map[string]any); ok {
		for key, value := range payload {
			result[key] = value
		}
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}
