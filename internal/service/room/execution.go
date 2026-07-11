package room

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
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
	service       *RealtimeService
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

func (s *RealtimeService) runSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentValue *protocol.Agent,
) {
	if agentValue == nil {
		slot.setStatus("error")
		s.loggerFor(ctx).Error("Room slot 缺少 agent 配置",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
		)
		return
	}

	slotCtx, cancel := context.WithCancel(ctx)
	slot.Cancel = cancel
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
	s.runtime.StartRound(slot.RuntimeSessionKey, slot.AgentRoundID, cancel)
	defer func() {
		s.runtime.MarkRoundFinished(slot.RuntimeSessionKey, slot.AgentRoundID)
		if !s.runtime.HasSubagentHistory(slot.RuntimeSessionKey) {
			if closeErr := s.runtime.CloseSession(context.Background(), slot.RuntimeSessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
				logger.Warn("关闭无 subagent 历史的 Room runtime 返回错误", "err", closeErr)
			}
		}
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
		ContextualInputs: goalContextualInputs(e.slot.GoalContext, e.slot.GoalIDForUsage, goalSessionKeyForSlot(e.slot)),
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
	dispatchPrompt, err := e.service.buildSlotVisibleContext(e.ctx, e.round, e.slot, e.history, e.agentNameByID, e.agent)
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
	if messageRole == "assistant" && roomdomain.IsNoReplyAssistantMessage(messageValue) {
		e.slot.suppressOutput()
		return nil
	}
	if e.slot.shouldSuppressOutput() {
		return nil
	}

	// 无回复标记只控制当前投递，不属于可持久化的对话正文。
	messageValue = roomdomain.StripNoReplyMarker(messageValue)
	if roomSlotPublishesPublicOutput(e.slot) {
		if err := e.service.persistSharedDurableMessage(e.round.ConversationID, e.slot, messageValue); err != nil {
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
