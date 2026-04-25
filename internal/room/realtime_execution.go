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
	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	usagesvc "github.com/nexus-research-lab/nexus/internal/usage"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"
)

func (s *RealtimeService) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*agent2.Agent,
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

	s.finishRound(roundValue.SessionKey)

	finalStatus := "finished"
	if roundValue.allSlotsCancelled() {
		finalStatus = "interrupted"
	} else if roundValue.hasSlotError() {
		finalStatus = "error"
	}
	logger.Info("Room round 结束", "status", finalStatus)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomRoundStatusEvent(
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
}

func (s *RealtimeService) runSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentValue *agent2.Agent,
) {
	if agentValue == nil {
		slot.Status = "error"
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
	mapper := newSlotMessageMapper(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, slot.AgentID, slot.MsgID, slot.AgentRoundID)
	slot.Status = "running"
	logger.Info("开始执行 Room slot")
	defer s.finishSlot(slot)

	s.permission.BindSessionRoute(slot.RuntimeSessionKey, permission3.RouteContext{
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
		mcpServers = s.mcpServers(agentValue.AgentID, slot.RuntimeSessionKey, "room")
	}
	permissionMode := sdkprotocol.PermissionMode(agentValue.Options.PermissionMode)
	permissionHandler := func(permissionCtx context.Context, request sdkprotocol.PermissionRequest) (sdkprotocol.PermissionDecision, error) {
		return s.permission.RequestPermission(permissionCtx, slot.RuntimeSessionKey, request)
	}
	options, err := runtimectx.BuildAgentClientOptions(slotCtx, s.providers, runtimectx.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           agentValue.Options.Provider,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       agentValue.Options.AllowedTools,
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    slot.SDKSessionID,
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
	})
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	previousStderr := options.Stderr
	options.Stderr = func(line string) {
		if previousStderr != nil {
			previousStderr(line)
		}
		logger.Warn("Agent SDK stderr", "stderr", line)
	}
	client := s.factory.New(options)
	slot.Client = client

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

	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamStart,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))

	trigger := slot.Trigger
	dispatchPrompt := buildRoomVisibleContext(roomVisibleContextInput{
		PublicHistory: history,
		LatestTrigger: trigger,
		AgentNameByID: agentNameByID,
		TargetAgentID: slot.AgentID,
	})
	if err := s.recordPrivateRoundMarker(slot, dispatchPrompt); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	slot.NoReplyCandidate = true
	result, err := runtimectx.ExecuteRound(slotCtx, runtimectx.RoundExecutionRequest{
		Query:  dispatchPrompt,
		Client: client,
		Mapper: roomRoundMapperAdapter{mapper: mapper},
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
				slot.Status = resultStatus(messageValue["subtype"])
				s.recordUsage(roundValue, messageValue)
			}
			if messageRole == "assistant" && isRoomNoReplyAssistantMessage(messageValue) {
				slot.SuppressOutput = true
				return nil
			}
			if slot.SuppressOutput {
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
			for _, readyEvent := range roomEventsReadyForEmission(slot, event) {
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

	if slot.Status == "running" {
		slot.Status = resultStatus(result.ResultSubtype)
	}
	if !slot.SuppressOutput {
		if err := s.collectPublicMentionWakes(slotCtx, roundValue, slot, mapper.LastAssistantMessage()); err != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
			return
		}
	}
	s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
	logger.Info("Room slot 结束", "status", slot.Status)
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
	if sessionID == "" || sessionID == strings.TrimSpace(slot.SDKSessionID) {
		return nil
	}
	slot.SDKSessionID = sessionID
	if s.rooms == nil {
		return nil
	}
	return s.rooms.UpdateSessionSDKSessionID(ctx, slot.RoomSessionID, sessionID)
}

func (s *RealtimeService) handleSlotFailure(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper, err error) {
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
	slot.Status = "error"
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
	projectedMessage := protocol.ProjectResultMessage(nil, resultMessage)
	if mapper != nil {
		projectedMessage = mapper.ProjectResultMessage(resultMessage)
	}
	s.broadcastSharedEventWithTimeout(
		ctx,
		roundValue.SessionKey,
		roundValue.RoomID,
		wrapRoomMessageEvent(
			roundValue.RoomID,
			roundValue.ConversationID,
			projectedMessage,
			slot.AgentRoundID,
		),
	)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, s.newRoomErrorEvent(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, "room_error", err.Error(), slot.AgentRoundID))
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func roomSlotFailureDiagnostics(err error, slot *activeRoomSlot, mapper *slotMessageMapper) []any {
	fields := make([]any, 0, 16)
	var streamClosed *runtimectx.RoundStreamClosedError
	if errors.As(err, &streamClosed) {
		fields = append(fields,
			"stream_messages_seen", streamClosed.MessagesSeen,
			"stream_last_type", streamClosed.LastMessageType,
			"stream_last_session_id", streamClosed.LastSessionID,
			"stream_last_message_id", streamClosed.LastMessageID,
			"stream_wait_error", streamClosed.WaitError,
		)
	}
	if mapper != nil {
		lastAssistant := mapper.LastAssistantMessage()
		fields = append(fields,
			"sdk_session_id", mapper.SessionID(),
			"current_message_id", mapper.CurrentMessageID(),
			"last_assistant_message_id", anyString(lastAssistant["message_id"]),
			"last_assistant_complete", lastAssistant["is_complete"],
			"last_assistant_chars", utf8.RuneCountInString(strings.TrimSpace(extractHistoryText(lastAssistant))),
		)
	}
	if slot != nil && slot.Client != nil {
		fields = append(fields, "client_session_id", slot.Client.SessionID())
	}
	return fields
}

func (s *RealtimeService) handleSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper) {
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
	)
	s.emitInterruptedSlotResult(roundValue, slot, mapper, "")
	s.broadcastSlotCancelled(ctx, roundValue, slot)
}

func (s *RealtimeService) markSlotCancelled(slot *activeRoomSlot) bool {
	if slot == nil || slot.Status == "cancelled" {
		return false
	}
	slot.Status = "cancelled"
	return true
}

func (s *RealtimeService) broadcastSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot) {
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, wrapRoomLifecycleEvent(
		protocol.EventTypeStreamCancelled,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func (s *RealtimeService) emitInterruptedSlotResult(roundValue *activeRoomRound, slot *activeRoomSlot, mapper *slotMessageMapper, resultText string) {
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
	if slot.Client != nil {
		if sessionID := strings.TrimSpace(slot.Client.SessionID()); sessionID != "" {
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
		projectedMessage := protocol.ProjectResultMessage(nil, resultMessage)
		if mapper != nil {
			projectedMessage = mapper.ProjectResultMessage(resultMessage)
		}
		s.broadcastSharedEvent(
			context.Background(),
			roundValue.SessionKey,
			roundValue.RoomID,
			wrapRoomMessageEvent(
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
	if sessionID := firstNonEmpty(strings.TrimSpace(anyString(privateMessage["session_id"])), strings.TrimSpace(slot.SDKSessionID)); sessionID != "" {
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
