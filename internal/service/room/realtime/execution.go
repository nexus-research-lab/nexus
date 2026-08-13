// INPUT: Room slot、trusted WorkBinding/ReviewBinding、运行时消息流、实时插话确认与 Goal 执行上下文。
// OUTPUT: 单个 Room Agent round 的 ACK 门控事件、持久化快照、usage barrier 与 producer root Attempt 终态。
// POS: Room 实时编排中把 runtime 输出投影为产品语义及结构化工作终态的执行主链。

package realtime

import (
	"cmp"
	"context"
	"errors"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
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
	service        *Service
	ctx            context.Context
	round          *activeRoomRound
	slot           *activeRoomSlot
	history        []protocol.Message
	agentNameByID  map[string]string
	agent          *protocol.Agent
	logger         *slog.Logger
	streamLogger   *slog.Logger
	mapper         *roomdomain.SlotMessageMapper
	emotionEnabled bool
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
		if settleErr := s.finishBoundRoomAttempt(
			ctx,
			roundValue,
			slot,
			"error",
			"Room slot 缺少 agent 配置",
		); settleErr != nil {
			s.loggerFor(ctx).Error(
				"Room structured root Attempt 缺少 Agent 配置时收口失败",
				"dispatch_id",
				executionDispatchID(slot.currentWorkBinding()),
				"err",
				settleErr,
			)
		}
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
	mapper.SetMessageDecorator(func(message protocol.Message) {
		s.decorateRoomMessage(roundValue, slot, message)
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

	routeLease := s.permission.BindSessionRoute(slot.RuntimeSessionKey, permissionctx.RouteContext{
		DispatchSessionKey: roundValue.SessionKey,
		RoomID:             roundValue.RoomID,
		ConversationID:     roundValue.ConversationID,
		AgentID:            slot.AgentID,
		MessageID:          slot.MsgID,
		RoundID:            roundValue.RootRoundID,
		AgentRoundID:       slot.AgentRoundID,
	})
	defer s.permission.UnbindSessionRoute(routeLease)

	admission, err := clientopts.BeginAgentRuntimeAdmission(
		execution.ctx,
		s.admission,
	)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, exec.RoundExecutionResult{}, err)
		return
	}
	defer admission.Release()
	execution.ctx = admission.Context()

	client, err := execution.prepareRuntimeClient()
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, exec.RoundExecutionResult{}, err)
		return
	}
	if err := s.runtime.StartRound(slotCtx, slot.RuntimeSessionKey, slot.AgentRoundID, cancel); err != nil {
		s.handleSlotFailure(
			slotCtx,
			roundValue,
			slot,
			mapper,
			exec.RoundExecutionResult{},
			err,
		)
		return
	}
	// session 与 round 已同时进入 Manager；后续认证转场可由 owner 级关闭完整撤销。
	execution.ctx = slotCtx
	admission.Release()
	defer func() {
		s.runtime.MarkRoundFinished(slot.RuntimeSessionKey, slot.AgentRoundID)
	}()
	defer execution.broadcastContextUsage(client)
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
			if partialErr := execution.persistInterruptedAssistant(); partialErr != nil {
				logger.Error("Room interrupted 流式内容持久化失败", "err", partialErr)
			}
			s.handleSlotCancelled(slotCtx, roundValue, slot, mapper, result)
			return
		}
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, result, err)
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
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, result, err)
		return
	}
	if err := s.ensureSlotOutputAuthorized(slotCtx, roundValue, slot); err != nil {
		s.retireSlotAfterOutputRevocation(slotCtx, roundValue, slot, err)
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

func (e *slotExecution) orchestrationActor() orchestration.ActorContext {
	actor := roomOrchestrationActor(e.round, e.slot)
	binding, bound := e.ensureWorkBindingState().Load()
	actor.WorkBinding = binding
	if bound {
		actor.ExecutionID = binding.ExecutionID
	}
	return actor
}

func (e *slotExecution) ensureWorkBindingState() *runtimectx.WorkBindingState {
	if e == nil || e.slot == nil {
		return nil
	}
	return e.slot.ensureWorkBindingState()
}

func roomOrchestrationActor(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) orchestration.ActorContext {
	goalAuthority := slot.goalMutationAuthority()
	actor := orchestration.ActorContext{
		OwnerUserID: roundValue.OwnerUserID,
		SessionKey:  roundValue.SessionKey,
		ExecutionID: firstNonEmptyString(
			executionIDFromRoomBindings(
				slot.WorkBinding,
				slot.ReviewBinding,
			),
			roundValue.ExecutionID,
		),
		WorkBinding:           cloneExecutionWorkBinding(slot.WorkBinding),
		ReviewBinding:         cloneExecutionReviewBinding(slot.ReviewBinding),
		GoalID:                strings.TrimSpace(goalAuthority.GoalID),
		GoalObjectiveRevision: goalAuthority.ObjectiveRevision,
		AgentID:               slot.AgentID,
		ActorKind:             protocol.ExecutionActorAgent,
		ScopeKind:             protocol.ExecutionScopeRoom,
		RoomID:                roundValue.RoomID,
		ConversationID:        roundValue.ConversationID,
		RootRoundID:           roundValue.RootRoundID,
		RuntimeRoundID:        slot.AgentRoundID,
		AgentRoundID:          slot.AgentRoundID,
		PlanMode:              roundValue.PermissionMode == sdkpermission.ModePlan,
	}
	actor.Role = roomExecutionActorRole(roundValue.CoordinatorAgentID, slot.AgentID)
	return actor
}

func (e *slotExecution) executeRound(client runtimectx.Client) (exec.RoundExecutionResult, error) {
	payload, err := e.prepareDispatchPayload()
	if err != nil {
		return exec.RoundExecutionResult{}, err
	}
	actor := e.orchestrationActor()
	defer e.service.releaseExecutionCoordination(actor)
	executionInputs, err := e.service.executionContextualInputs(e.ctx, actor)
	if err != nil {
		return exec.RoundExecutionResult{}, err
	}
	if e.service.subagentAdmission != nil {
		e.service.runtime.SetSubagentHookCallbacks(
			e.slot.RuntimeSessionKey,
			e.slot.AgentRoundID,
			orchestrationruntimehook.Callbacks(
				e.service.subagentAdmission,
				orchestrationruntimehook.Context{
					Actor:             actor,
					ActorProvider:     e.orchestrationActor,
					RuntimeSessionKey: e.slot.RuntimeSessionKey,
					RoomSessionID:     e.slot.RoomSessionID,
					Logger:            e.service.loggerFor(e.ctx),
				},
			),
		)
		defer e.service.runtime.ClearSubagentHookCallbacks(
			e.slot.RuntimeSessionKey,
			e.slot.AgentRoundID,
		)
	}
	e.slot.beginNoReplyCandidate()
	e.service.beginExecutionRuntimeGraph(actor)
	result, executeErr := exec.ExecuteRound(e.ctx, exec.RoundExecutionRequest{
		Content:          payload,
		ContextualInputs: append(executionInputs, e.contextualInputs()...),
		InputOptions:     roomSlotRuntimeInputOptions(e.round, e.slot),
		Client:           client,
		Mapper:           roomRoundMapperAdapter{mapper: e.mapper},
		IdleTimeout:      e.service.config.RuntimeRoundIdleTimeout(),
		IdlePauseState: func() (bool, <-chan struct{}) {
			return e.service.permission.PendingRequestState(e.slot.RuntimeSessionKey)
		},
		InterruptReason: func() string {
			return roomSlotInterruptReason(e.slot)
		},
		AfterQuery: func() error {
			if err := e.activateBoundRoomAttempt(actor); err != nil {
				return err
			}
			return e.sendQueuedInputs(client)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			currentActor := e.orchestrationActor()
			e.service.observeExecutionRuntimeGraph(currentActor, incoming)
			e.observeExecutionPersistenceEvidence(currentActor, incoming)
			e.observeIncomingMessage(incoming)
		},
		SyncSessionID: func(sessionID string) error {
			return e.service.syncSlotSDKSessionID(e.ctx, e.slot, sessionID)
		},
		HandleDurableMessage: e.handleDurableMessage,
		EmitEvent:            e.emitEvent,
	})
	failureReason := ""
	if executeErr != nil {
		failureReason = executeErr.Error()
	}
	e.service.finishExecutionRuntimeGraph(
		e.orchestrationActor(),
		result.TerminalStatus,
		failureReason,
	)
	return result, executeErr
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
	runtimeContent = e.service.appendRuntimeUserContext(
		e.ctx,
		e.round.ConversationID,
		e.agent,
		runtimeContent,
		e.emotionEnabled,
	)
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
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
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
	settledSubagentUsage := e.service.recordSubagentGoalUsageForSlot(e.ctx, e.slot, messageValue)
	e.slot.rememberSubagentTaskMessage(messageValue)
	for _, settlement := range settledSubagentUsage {
		e.slot.clearSubagentUsageObservationPending(settlement.taskID, settlement.observation)
	}
	e.service.startRoomSubagentUsageRetry(e.round, e.slot)
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
	// 旧版 fanout 标记只做输入兼容清理，不应泄漏到 transcript、私域 overlay 或公区。
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
		if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
			return err
		}
		if err := e.service.persistPrivateOverlayMessage(e.slot, cloneMessageWithSessionKey(messageValue, e.slot.RuntimeSessionKey)); err != nil {
			return err
		}
	}
	if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
		return err
	}
	e.service.observeExecutionRuntimeArtifacts(e.orchestrationActor(), messageValue)
	e.service.recordGoalUsageFromSlotAssistantMessage(e.ctx, e.slot, messageValue)
	return nil
}

func (e *slotExecution) persistInterruptedAssistant() error {
	partial := e.mapper.FinalizeInterruptedAssistant()
	if len(partial) == 0 {
		return nil
	}
	return e.handleDurableMessage(partial)
}

func (e *slotExecution) emitEvent(event protocol.EventMessage) error {
	if roomSlotShouldDropPublicOutputEvent(e.slot, event) {
		return nil
	}
	if event.EventType == protocol.EventTypeMessage {
		if err := e.service.ensureSlotOutputAuthorized(e.ctx, e.round, e.slot); err != nil {
			return err
		}
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

	if !s.settleCompletedRoomGoalUsage(ctx, roundValue) {
		logger.Warn(
			"Room Goal usage 尚未完成最终结算",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
		)
	}
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
			s.dispatchPostRoundWorkOnce(taskCtx, roundValue)
		},
	)
}

// settleCompletedRoomGoalUsage 在所有 parent slot 结束后建立最终 usage barrier。
// 同步 settlement/finalization 耗尽时，后台 worker 会持续重试；即使没有
// child pending，也不会依赖下一条 runtime 消息才能恢复。
func (s *Service) settleCompletedRoomGoalUsage(
	ctx context.Context,
	roundValue *activeRoomRound,
) bool {
	if s == nil || roundValue == nil {
		return true
	}
	roundValue.RunningSubagents.Store(roundValue.hasRunningSubagentTasks())
	if s.finalizeCompletedRoomGoalUsage(ctx, roundValue) {
		return true
	}

	// RunningSubagents 同时承担 post-round settlement barrier。worker 成功后
	// 通过 CAS 释放；runRound 的普通收尾因该 barrier 不会提前派发。
	roundValue.RunningSubagents.Store(true)
	var coordinator *activeRoomSlot
	for _, slot := range roundValue.Slots {
		if slot == nil {
			continue
		}
		if len(slot.subagentUsagePendingSnapshot()) > 0 {
			s.startRoomSubagentUsageRetry(roundValue, slot)
			if coordinator == nil {
				coordinator = slot
			}
			continue
		}
		if coordinator == nil && slot.goalUsageSettlementRequired() {
			coordinator = slot
		}
	}
	if coordinator != nil {
		s.startRoomGoalUsageRetry(roundValue, coordinator)
	}
	if roundValue.postRoundDispatched.Load() {
		// 一个并发 child worker 已经完成同一 settlement 并释放 post-round。
		roundValue.RunningSubagents.Store(false)
		return true
	}
	return false
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

// roomSlotRuntimeInputOptions 让 Recall 只搜索直接唤醒该 slot 的原始语义。
func roomSlotRuntimeInputOptions(roundValue *activeRoomRound, slot *activeRoomSlot) sdkprotocol.OutboundMessageOptions {
	options := runtimectx.RuntimeInputOptionsForPurpose(roomRoundInputOptions(roundValue), "goal_continuation")
	options.RecallQuery = ""
	if roundValue == nil || slot == nil || roundValue.Internal ||
		options.Meta || options.Synthetic || options.HiddenFromUser {
		return options
	}
	options.RecallQuery = strings.TrimSpace(slot.Trigger.Content)
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
