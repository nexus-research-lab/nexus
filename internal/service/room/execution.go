package room

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
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
	go s.dispatchPostRoundWork(contextWithQueueOwner(context.Background(), roundValue.OwnerUserID), roundValue)
}

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
		slot.AgentRoundID,
		agentValue.WorkspacePath,
	)
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
	appendSystemPrompt = appendPromptSection(appendSystemPrompt, roomdomain.BuildSystemPrompt())
	appendSystemPrompt = appendPromptSection(appendSystemPrompt, s.buildRoomMemorySystemPrompt(slotCtx, roundValue))
	roomSkillPrompt, err := s.rooms.BuildRoomSkillPrompt(slotCtx, roundValue.Context.Room.SkillNames)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	appendSystemPrompt = appendPromptSection(appendSystemPrompt, roomSkillPrompt)
	appendSystemPrompt = appendPromptSection(appendSystemPrompt, roomdomain.BuildMemberDirectoryPrompt(agentNameByID))
	permissionMode := sdkpermission.Mode(agentValue.Options.PermissionMode)
	if roundValue.PermissionMode != "" {
		permissionMode = roundValue.PermissionMode
	}
	slot.GoalRuntimeIgnored = goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode))
	if !slot.GoalRuntimeIgnored {
		appendSystemPrompt, slot.GoalContext, slot.GoalIDForUsage, slot.GoalSessionKey = s.resolveGoalRuntimeContextForSlot(slotCtx, roundValue, slot, appendSystemPrompt)
	}
	if override := strings.TrimSpace(roundValue.GoalContext); roundValue.Internal && override != "" {
		slot.GoalContext = override
	}
	beginGoalUsageForSlot(slot)
	cleanupGoalRuntime := s.registerSlotGoalRuntime(slot)
	defer cleanupGoalRuntime()
	mcpServers := map[string]sdkmcp.ServerConfig(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(
			agentValue.AgentID,
			roundValue.SessionKey,
			slot.AgentRoundID,
			"room",
			roundValue.RoomID,
			roomSourceContextLabel(roundValue),
		)
	}
	permissionHandler := roundValue.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
			return s.permission.RequestPermission(permissionCtx, slot.RuntimeSessionKey, request)
		}
	}
	permissionHandler = roomRuntimePermissionHandler(permissionHandler)
	permissionHandler = toolpolicy.WithManagedGoalAutoApproval(permissionHandler)
	runtimeSelection, err := s.resolveAgentRuntimeSelection(slotCtx, roundValue, agentValue)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	options, err := clientopts.BuildAgentClientOptions(slotCtx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              agentValue.WorkspacePath,
		RuntimeKind:                runtimeSelection.RuntimeKind,
		Provider:                   runtimeSelection.Provider,
		Model:                      runtimeSelection.Model,
		PermissionMode:             permissionMode,
		PermissionHandler:          permissionHandler,
		AllowedTools:               toolpolicy.WithManagedGoalAllowedTools(roomRuntimeAllowedTools(agentValue.Options.AllowedTools)),
		DisallowedTools:            roomRuntimeDisallowedTools(agentValue.Options.DisallowedTools),
		SettingSources:             agentValue.Options.SettingSources,
		AppendSystemPrompt:         appendSystemPrompt,
		ResumeSessionID:            slot.getSDKSessionID(),
		MaxThinkingTokens:          agentValue.Options.MaxThinkingTokens,
		MaxTurns:                   agentValue.Options.MaxTurns,
		MCPServers:                 mcpServers,
		ExtraEnv:                   s.roomRuntimeEnv(roundValue, slot),
		AgentSDKDiagnosticsEnabled: runtimeSelection.AgentSDKDiagnosticsEnabled,
	})
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	options = s.runtime.WithGuidanceHook(options, slot.RuntimeSessionKey)
	if goalSessionKey := goalSessionKeyForSlot(slot); goalSessionKey != "" && goalSessionKey != slot.RuntimeSessionKey {
		options = s.runtime.WithGuidanceHook(options, goalSessionKey)
	}
	options = runtimectx.WithPostToolUseGuidanceHook(options, s.roomSlotGuidanceHook(roundValue, slot, workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  agentValue.WorkspacePath,
		SessionKey:     slot.RuntimeSessionKey,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
	}))
	options = withRoomRuntimeDiagnosticsLogger(options, logger.With("agent_id", slot.AgentID, "agent_round_id", slot.AgentRoundID))
	runtimeProvider := clientopts.ResolvedRuntimeProvider(runtimeSelection.Provider, options)
	logger.Info("准备启动 Room runtime",
		append(clientopts.RuntimeStartupLogFields(options),
			"agent_id", slot.AgentID,
			"agent_round_id", slot.AgentRoundID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"requested_runtime_kind", strings.TrimSpace(runtimeSelection.RuntimeKind),
			"requested_provider", strings.TrimSpace(runtimeSelection.Provider),
			"requested_model", strings.TrimSpace(runtimeSelection.Model),
			"runtime_provider", runtimeProvider,
		)...,
	)
	client := s.factory.New(options)
	slot.setClient(client)

	if err := client.Connect(slotCtx); err != nil {
		logger.Error("Room runtime 启动失败",
			append(clientopts.RuntimeStartupLogFields(options),
				"agent_id", slot.AgentID,
				"agent_round_id", slot.AgentRoundID,
				"runtime_session_key", slot.RuntimeSessionKey,
				"stage", "connect",
				"err", err,
				"error_type", fmt.Sprintf("%T", err),
				"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
			)...,
		)
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	logger.Info("Room runtime 启动成功",
		append(clientopts.RuntimeStartupLogFields(options),
			"agent_id", slot.AgentID,
			"agent_round_id", slot.AgentRoundID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"sdk_session_id", strings.TrimSpace(client.SessionID()),
		)...,
	)
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

	dispatchPrompt, err := s.buildSlotVisibleContext(slotCtx, roundValue, slot, history, agentNameByID, agentValue)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	if err := s.recordPrivateRoundMarker(roundValue, slot, dispatchPrompt); err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	dispatchRuntimeContent, err := s.renderRuntimeContentWithAttachments(slotCtx, dispatchPrompt, slot.TriggerAttachments)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	dispatchRuntimeContent = s.appendRuntimeUserContext(slotCtx, roundValue.ConversationID, agentValue, dispatchRuntimeContent)
	slot.beginNoReplyCandidate()
	result, err := runtimectx.ExecuteRound(slotCtx, runtimectx.RoundExecutionRequest{
		Content:          dispatchRuntimeContent.Payload(),
		ContextualInputs: goalContextualInputs(slot.GoalContext, slot.GoalIDForUsage, goalSessionKeyForSlot(slot)),
		InputOptions:     runtimectx.RuntimeInputOptionsForPurpose(roomRoundInputOptions(roundValue), "goal_continuation"),
		Client:           client,
		Mapper:           roomRoundMapperAdapter{mapper: mapper},
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
					"content_preview", logx.PreviewText(input.Content, 240),
				)
			}
			return nil
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			if streamLogger.Enabled(slotCtx, slog.LevelDebug) {
				if incoming.Type == sdkprotocol.MessageTypeStreamEvent && !s.config.MessageDebugStreamEvent {
					return
				}
				streamLogger.Debug(
					"Room slot 收到 SDK 消息",
					runtimectx.BuildSDKMessageLogFieldsWithOptions(
						incoming,
						runtimectx.SDKMessageLogOptions{
							IncludeStreamEvent:  s.config.MessageDebugStreamEvent,
							IncludeSnapshotData: true,
						},
					)...,
				)
			}
		},
		SyncSessionID: func(sessionID string) error {
			return s.syncSlotSDKSessionID(slotCtx, slot, sessionID)
		},
		HandleDurableMessage: func(messageValue protocol.Message) error {
			messageRole := protocol.MessageRole(messageValue)
			if messageRole == "result" {
				slot.setStatus(resultStatus(messageValue["subtype"]))
				s.recordUsage(roundValue, slot, messageValue)
			}
			if messageRole == "assistant" {
				slot.rememberGoalAssistantMessage(messageValue)
			}
			if messageRole == "assistant" && roomdomain.IsNoReplyAssistantMessage(messageValue) {
				slot.suppressOutput()
				return nil
			}
			if slot.shouldSuppressOutput() {
				return nil
			}
			if !roomSlotPublishesPublicOutput(slot) {
				if !protocol.IsTranscriptNativeMessage(protocol.Message(messageValue)) {
					if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
						return err
					}
				}
				s.recordGoalUsageFromSlotAssistantMessage(slotCtx, slot, messageValue)
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
			s.recordGoalUsageFromSlotAssistantMessage(slotCtx, slot, messageValue)
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			if roomSlotShouldDropPublicOutputEvent(slot, event) {
				return nil
			}
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

	if result.CompletedByAssistant {
		s.recordTerminalAssistantUsage(roundValue, slot, mapper.LastAssistantMessage())
	}
	s.recordGoalUsageForSlot(slotCtx, slot, result, mapper.LastAssistantMessage())
	s.recordGoalUsageLimitForSlot(slotCtx, slot, result)
	s.recordGoalContinuationProgressForSlot(slotCtx, slot, roundValue, result, mapper.LastAssistantMessage())
	if slot.getStatus() == "running" {
		slot.setStatus(resultStatus(result.ResultSubtype))
	}
	if !slot.shouldSuppressOutput() {
		if err := s.recordRoomDirectedMessageReply(slotCtx, roundValue, slot, mapper.LastAssistantMessage()); err != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
			return
		}
		if roomSlotPublishesPublicOutput(slot) {
			if err := s.collectPublicMentionWakes(slotCtx, roundValue, slot, mapper.LastAssistantMessage()); err != nil {
				s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
				return
			}
		}
	}
	if slot.getStatus() == "finished" {
		if err := s.recordRoomPublicCursor(slot, roundValue, slot.PublicCursorID, slot.PublicCursorTS); err != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
			return
		}
		messageCursor, messageCursorRecorded, err := s.recordRoomDirectedMessageCursor(slot, roundValue)
		if err != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
			return
		}
		if messageCursorRecorded {
			s.broadcastSharedEventWithTimeout(
				slotCtx,
				roundValue.SessionKey,
				roundValue.RoomID,
				newRoomDirectedMessageConsumedEvent(messageCursor),
			)
		}
	}
	if result.CompletedByAssistant && roomSlotCanCommitMemory(slot) {
		go s.commitRoomMemoryTurn(roundValue, slot, mapper.LastAssistantMessage())
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

func withRoomRuntimeDiagnosticsLogger(options agentclient.Options, logger *slog.Logger) agentclient.Options {
	previousStderr := options.Callbacks.Stderr
	options.Callbacks.Stderr = func(line string) {
		normalizedLine := normalizeRuntimeStderrLine(line)
		if previousStderr != nil {
			previousStderr(normalizedLine)
		}
		logger.Warn("Agent SDK stderr", "stderr", normalizedLine)
	}
	previousDiagnostics := options.Callbacks.Diagnostics
	diagnosticsEnabled := runtimectx.AgentSDKDiagnosticsEnabled(options.Env)
	options.Callbacks.Diagnostics = func(event agentclient.DiagnosticEvent) {
		if previousDiagnostics != nil {
			previousDiagnostics(event)
		}
		if diagnosticsEnabled {
			logger.Info("Agent SDK diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldLogRuntimeStartupDiagnostic(event) {
			logger.Info("Agent SDK startup diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldWarnRuntimeStartupDiagnostic(event) {
			logger.Warn("Agent SDK startup diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
		}
	}
	if !diagnosticsEnabled {
		return options
	}
	logger.Info("Agent SDK diagnostics 已启用",
		"diagnostics_env", runtimectx.AgentSDKDiagnosticsValue(options.Env),
		"provider_debug_body", runtimectx.AgentSDKProviderDebugBodyValue(options.Env),
	)
	return options
}

func roomSourceContextLabel(roundValue *activeRoomRound) string {
	if roundValue == nil || roundValue.Context == nil {
		return ""
	}
	if roomName := strings.TrimSpace(roundValue.Context.Room.Name); roomName != "" {
		return roomName
	}
	return strings.TrimSpace(roundValue.Context.Conversation.Title)
}

func roomRuntimeAllowedTools(values []string) []string {
	if len(toolpolicy.NormalizeSet(values)) == 0 {
		return values
	}
	return appendDistinctRoomRuntimeTools(values, "nexus_room")
}

func roomRuntimeDisallowedTools(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if isRoomRuntimeTool(value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func appendDistinctRoomRuntimeTools(values []string, extra ...string) []string {
	result := make([]string, 0, len(values)+len(extra))
	seen := make(map[string]struct{}, len(values)+len(extra))
	for _, value := range append(append([]string(nil), values...), extra...) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func roomRuntimePermissionHandler(next sdkpermission.Handler) sdkpermission.Handler {
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if isRoomRuntimeTool(request.ToolName) {
			return sdkpermission.Allow(request.Input, nil), nil
		}
		if next == nil {
			return sdkpermission.Allow(request.Input, nil), nil
		}
		return next(ctx, request)
	}
}

func isRoomRuntimeTool(toolName string) bool {
	normalized := strings.TrimSpace(toolName)
	return normalized == "nexus_room" ||
		strings.HasPrefix(normalized, "mcp__nexus_room__") ||
		strings.HasPrefix(normalized, "nexus_room__") ||
		strings.HasPrefix(normalized, "nexus_room.")
}
