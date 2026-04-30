package room

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
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
		}(slot)
	}
	waitGroup.Wait()

	s.finishRound(roundValue)

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
	if finalStatus == "finished" {
		s.startQueuedPublicMentionWakes(context.Background(), roundValue)
	}
	go s.dispatchNextInputQueueItem(
		contextWithQueueOwner(context.Background(), roundValue.OwnerUserID),
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
	)
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
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"round_id", slot.AgentRoundID,
			"agent_id", slot.AgentID,
			"msg_id", slot.MsgID,
		)
		return
	}

	slotCtx, cancel := context.WithCancel(ctx)
	slot.Cancel = cancel
	logger := s.loggerFor(slotCtx).With(
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
	)
	mapper := roomdomain.NewSlotMessageMapper(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, slot.AgentID, slot.MsgID, slot.AgentRoundID)
	slot.setStatus("running")
	logger.Info("开始执行 Room slot")
	defer s.finishSlot(slot)

	s.permission.BindSessionRoute(slot.RuntimeSessionKey, permissionctx.RouteContext{
		DispatchSessionKey: roundValue.SessionKey,
		RoomID:             roundValue.RoomID,
		ConversationID:     roundValue.ConversationID,
		AgentID:            slot.AgentID,
		MessageID:          slot.MsgID,
		CausedBy:           slot.AgentRoundID,
	})
	defer s.permission.UnbindSessionRoute(slot.RuntimeSessionKey)

	if err := workspacepkg.EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	appendSystemPrompt, err := s.agents.BuildRuntimePrompt(slotCtx, agentValue)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	mcpServers := map[string]agentclient.SDKMCPServer(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(agentValue.AgentID, slot.RuntimeSessionKey, "room", roundValue.RoomID, "")
	}
	permissionMode := sdkprotocol.PermissionMode(agentValue.Options.PermissionMode)
	permissionHandler := func(permissionCtx context.Context, request sdkprotocol.PermissionRequest) (sdkprotocol.PermissionDecision, error) {
		return s.permission.RequestPermission(permissionCtx, slot.RuntimeSessionKey, request)
	}
	options, err := clientopts.BuildAgentClientOptions(slotCtx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           agentValue.Options.Provider,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       agentValue.Options.AllowedTools,
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    slot.getSDKSessionID(),
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
	})
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	options = runtimectx.WithPostToolUseGuidanceHook(options, s.roomSlotGuidanceHook(roundValue, slot, workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     slot.RuntimeSessionKey,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	}))
	previousStderr := options.Stderr
	options.Stderr = func(line string) {
		if previousStderr != nil {
			previousStderr(line)
		}
		logger.Warn("Agent SDK stderr", "stderr", line)
	}
	client := s.factory.New(options)
	slot.setClient(client)

	if err := client.Connect(slotCtx); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Warn("Agent SDK disconnect 返回错误", "err", err)
		}
	}()
	if err := s.syncSlotSDKSessionID(slotCtx, slot, client.SessionID()); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamStart,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))

	trigger := slot.Trigger
	dispatchPrompt := roomdomain.BuildVisibleContext(roomdomain.VisibleContextInput{
		PublicHistory: history,
		LatestTrigger: trigger,
		AgentNameByID: agentNameByID,
		TargetAgentID: slot.AgentID,
	})
	if err := s.recordPrivateRoundMarker(slot, dispatchPrompt); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	slot.beginNoReplyCandidate()
	result, err := runtimectx.ExecuteRound(slotCtx, runtimectx.RoundExecutionRequest{
		Query:  dispatchPrompt,
		Client: client,
		Mapper: roomRoundMapperAdapter{mapper: mapper},
		InterruptReason: func() string {
			return roomSlotInterruptReason(slot)
		},
		AfterQuery: func() error {
			for _, input := range slot.drainQueuedInputs() {
				if err := runtimectx.SendClientContent(slotCtx, client, input.Content); err != nil {
					return err
				}
				logger.Info("发送已排队的 Room 消息",
					"queued_round_id", input.RoundID,
					"content_chars", utf8.RuneCountInString(input.Content),
				)
			}
			return nil
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			if logger.Enabled(slotCtx, slog.LevelDebug) {
				logger.Debug("Agent ", runtimectx.BuildSDKMessageLogFields(incoming)...)
			}
		},
		SyncSessionID: func(sessionID string) error {
			return s.syncSlotSDKSessionID(slotCtx, slot, sessionID)
		},
		HandleDurableMessage: func(messageValue protocol.Message) error {
			messageRole := protocol.MessageRole(messageValue)
			if messageRole == "result" {
				slot.setStatus(resultStatus(messageValue["subtype"]))
				s.recordUsage(roundValue, messageValue)
			}
			if messageRole == "assistant" && roomdomain.IsNoReplyAssistantMessage(messageValue) {
				slot.suppressOutput()
				return nil
			}
			if slot.shouldSuppressOutput() {
				return nil
			}
			if err := s.persistSharedDurableMessage(roundValue.ConversationID, slot, messageValue); err != nil {
				return err
			}
			if !protocol.IsTranscriptNativeMessage(protocol.Message(messageValue)) {
				if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
					return err
				}
			}
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			for _, readyEvent := range slot.eventsReadyForEmission(event) {
				s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, readyEvent)
			}
			return nil
		},
	})
	if err != nil {
		if errors.Is(err, runtimectx.ErrRoundInterrupted) {
			s.handleSlotCancelled(slotCtx, roundValue, slot, mapper)
			return
		}
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}

	if slot.getStatus() == "running" {
		slot.setStatus(resultStatus(result.ResultSubtype))
	}
	if !slot.shouldSuppressOutput() {
		if err := s.collectPublicMentionWakes(slotCtx, roundValue, slot, mapper.LastAssistantMessage()); err != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
			return
		}
	}
	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
	logger.Info("Room slot 结束", "status", slot.getStatus())
}

func (s *RealtimeService) recordUsage(roundValue *activeRoomRound, message protocol.Message) {
	if s.usage == nil || roundValue == nil || protocol.MessageRole(message) != "result" {
		return
	}
	input := usagesvc.MessageRecordInput(roundValue.OwnerUserID, "room_runtime", message)
	if err := s.usage.RecordMessageUsage(context.Background(), input); err != nil {
		s.loggerFor(context.Background()).Error("Room token usage 写入失败",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"round_id", roundValue.RoundID,
			"err", err,
		)
	}
}

func (s *RealtimeService) syncSlotSDKSessionID(ctx context.Context, slot *activeRoomSlot, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if !slot.setSDKSessionID(sessionID) {
		return nil
	}
	if s.rooms == nil {
		return nil
	}
	return s.rooms.UpdateSessionSDKSessionID(ctx, slot.RoomSessionID, sessionID)
}

func (s *RealtimeService) handleSlotFailure(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper, err error) {
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
	s.loggerFor(ctx).Error("Room slot 执行失败", fields...)
	slot.setStatus("error")
	resultMessage := protocol.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "error",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"result":          err.Error(),
		"is_error":        true,
		"timestamp":       time.Now().UnixMilli(),
	}
	_ = s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage)
	_ = s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey))
	projectedMessage := message.ProjectResultMessage(nil, resultMessage)
	if mapper != nil {
		projectedMessage = mapper.ProjectResultMessage(resultMessage)
	}
	s.broadcastSharedEventWithTimeout(
		ctx,
		roundValue.SessionKey,
		roundValue.RoomID,
		roomdomain.WrapMessageEvent(
			roundValue.RoomID,
			roundValue.ConversationID,
			projectedMessage,
			slot.AgentRoundID,
		),
	)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.NewErrorEvent(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, "room_error", err.Error(), slot.AgentRoundID))
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func roomSlotFailureDiagnostics(err error, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper) []any {
	fields := make([]any, 0, 16)
	var streamClosed *runtimectx.RoundStreamClosedError
	if errors.As(err, &streamClosed) {
		fields = append(fields,
			"stream_messages_seen", streamClosed.MessagesSeen,
			"stream_last_type", streamClosed.LastMessageType,
			"stream_last_summary", streamClosed.LastMessageSummary,
			"stream_last_session_id", streamClosed.LastSessionID,
			"stream_last_message_id", streamClosed.LastMessageID,
			"stream_wait_error", streamClosed.WaitError,
		)
	}
	var streamIdle *runtimectx.RoundStreamIdleTimeoutError
	if errors.As(err, &streamIdle) {
		fields = append(fields,
			"stream_idle_timeout", streamIdle.IdleTimeout.String(),
			"stream_messages_seen", streamIdle.MessagesSeen,
			"stream_last_type", streamIdle.LastMessageType,
			"stream_last_summary", streamIdle.LastMessageSummary,
			"stream_last_session_id", streamIdle.LastSessionID,
			"stream_last_message_id", streamIdle.LastMessageID,
		)
	}
	if mapper != nil {
		lastAssistant := mapper.LastAssistantMessage()
		fields = append(fields,
			"sdk_session_id", mapper.SessionID(),
			"current_message_id", mapper.CurrentMessageID(),
			"last_assistant_message_id", anyString(lastAssistant["message_id"]),
			"last_assistant_complete", lastAssistant["is_complete"],
			"last_assistant_chars", utf8.RuneCountInString(strings.TrimSpace(roomdomain.ExtractHistoryText(lastAssistant))),
		)
	}
	if client := slot.getClient(); client != nil {
		fields = append(fields, "client_session_id", client.SessionID())
	}
	return fields
}

func (s *RealtimeService) handleSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper) {
	if !s.markSlotCancelled(slot) {
		return
	}
	s.loggerFor(ctx).Warn("Room slot 已取消",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"reason", roomSlotInterruptReason(slot),
	)
	s.emitInterruptedSlotResult(roundValue, slot, mapper, "")
	s.broadcastSlotCancelled(ctx, roundValue, slot)
}

func (s *RealtimeService) markSlotCancelled(slot *activeRoomSlot) bool {
	return slot != nil && slot.markCancelled()
}

func (s *RealtimeService) broadcastSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot) {
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamCancelled,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func (s *RealtimeService) emitInterruptedSlotResult(roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper, resultText string) {
	if roundValue == nil || slot == nil {
		return
	}
	resultMessage := protocol.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"is_error":        false,
		"timestamp":       time.Now().UnixMilli(),
	}
	if trimmedResult := strings.TrimSpace(resultText); trimmedResult != "" {
		resultMessage["result"] = trimmedResult
	}
	if client := slot.getClient(); client != nil {
		if sessionID := strings.TrimSpace(client.SessionID()); sessionID != "" {
			resultMessage["session_id"] = sessionID
		}
	}
	if err := s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage); err != nil {
		s.loggerFor(context.Background()).Error("Room interrupted 共享结果持久化失败",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"err", err,
		)
	} else {
		projectedMessage := message.ProjectResultMessage(nil, resultMessage)
		if mapper != nil {
			projectedMessage = mapper.ProjectResultMessage(resultMessage)
		}
		s.broadcastSharedEvent(
			context.Background(),
			roundValue.SessionKey,
			roundValue.RoomID,
			roomdomain.WrapMessageEvent(
				roundValue.RoomID,
				roundValue.ConversationID,
				projectedMessage,
				slot.AgentRoundID,
			),
		)
	}
	if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey)); err != nil {
		s.loggerFor(context.Background()).Error("Room interrupted 私有结果持久化失败",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"err", err,
		)
	}
}

func (s *RealtimeService) recordPrivateRoundMarker(slot *activeRoomSlot, dispatchPrompt string) error {
	if s.history == nil {
		return nil
	}
	return s.history.AppendRoundMarker(
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		slot.AgentRoundID,
		strings.TrimSpace(dispatchPrompt),
		time.Now().UnixMilli(),
	)
}

func (s *RealtimeService) persistPrivateOverlayMessage(slot *activeRoomSlot, message protocol.Message) error {
	if s.history == nil {
		return nil
	}
	privateMessage := normalizePrivateOverlayMessage(cloneMessageWithSessionKey(message, slot.RuntimeSessionKey))
	privateMessage["session_key"] = slot.RuntimeSessionKey
	if sessionID := firstNonEmpty(strings.TrimSpace(anyString(privateMessage["session_id"])), slot.getSDKSessionID()); sessionID != "" {
		privateMessage["session_id"] = sessionID
	}
	if strings.TrimSpace(anyString(privateMessage["message_id"])) == "" {
		privateMessage["message_id"] = "overlay_" + slot.AgentRoundID
	}
	privateMessage["metadata"] = mergePrivateOverlayMetadata(privateMessage["metadata"], map[string]any{
		"overlay_source":  "room_runtime",
		"room_session_id": slot.RoomSessionID,
	})
	return s.history.AppendOverlayMessage(slot.WorkspacePath, slot.RuntimeSessionKey, privateMessage)
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
