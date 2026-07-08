package room

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/room/runtimepolicy"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
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
	appendSystemPrompt = appendPromptSection(appendSystemPrompt, roomdomain.BuildSystemPrompt(
		roundValue.Context.Room.PrivateMessagesEnabled,
	))
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
			roundValue.RootRoundID,
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
	permissionHandler = runtimepolicy.PermissionHandler(permissionHandler, roundValue.Context.Room.PrivateMessagesEnabled)
	permissionHandler = toolpolicy.WithManagedGoalAutoApproval(permissionHandler)
	permissionHandler = toolpolicy.WithMalformedInputDeny(permissionHandler)
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
		AllowedTools:               toolpolicy.WithManagedRuntimeAllowedTools(runtimepolicy.AllowedTools(agentValue.Options.AllowedTools, roundValue.Context.Room.PrivateMessagesEnabled), s.runtimeImagegenDefaultEnabled(slotCtx)),
		DisallowedTools:            runtimepolicy.DisallowedTools(agentValue.Options.DisallowedTools, roundValue.Context.Room.PrivateMessagesEnabled),
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
	resumeID, err := s.resolveReusableRoomSDKSessionID(
		slotCtx,
		logger,
		agentValue.WorkspacePath,
		slot,
		options.Session.ResumeID,
	)
	if err != nil {
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	options.Session.ResumeID = resumeID
	connectClient := func(currentOptions agentclient.Options) (runtimectx.Client, error) {
		logger.Info("准备启动 Room runtime",
			roomRuntimeStartupLogFields(currentOptions, runtimeSelection, runtimeProvider, slot)...,
		)
		currentClient := s.factory.New(currentOptions)
		slot.setClient(currentClient)
		return currentClient, currentClient.Connect(slotCtx)
	}

	client, err := connectClient(options)
	if err != nil && strings.TrimSpace(options.Session.ResumeID) != "" && runtimectx.IsRuntimeTransportClosedError(err) {
		logger.Warn("Room SDK session resume 失效，清除后重试",
			append(roomRuntimeConnectFailureLogFields(options, runtimeSelection, runtimeProvider, slot, err),
				"sdk_session_id", strings.TrimSpace(options.Session.ResumeID),
			)...,
		)
		if client != nil {
			if disconnectErr := client.Disconnect(context.Background()); disconnectErr != nil && !runtimectx.IsRuntimeTransportClosedError(disconnectErr) {
				s.handleSlotFailure(slotCtx, roundValue, slot, mapper, disconnectErr)
				return
			}
		}
		if clearErr := s.clearSlotSDKSessionID(slotCtx, slot); clearErr != nil {
			s.handleSlotFailure(slotCtx, roundValue, slot, mapper, clearErr)
			return
		}
		options.Session.ResumeID = ""
		client, err = connectClient(options)
	}
	if err != nil {
		logger.Error("Room runtime 启动失败", roomRuntimeConnectFailureLogFields(options, runtimeSelection, runtimeProvider, slot, err)...)
		s.handleSlotFailure(slotCtx, roundValue, slot, mapper, err)
		return
	}
	logger.Info("Room runtime 启动成功",
		append(roomRuntimeStartupLogFields(options, runtimeSelection, runtimeProvider, slot),
			"sdk_session_id", strings.TrimSpace(client.SessionID()),
		)...,
	)
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Warn("Agent SDK disconnect 返回错误", "err", err)
		}
	}()

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
	result, err := exec.ExecuteRound(slotCtx, exec.RoundExecutionRequest{
		Content:          dispatchRuntimeContent.Payload(),
		ContextualInputs: goalContextualInputs(slot.GoalContext, slot.GoalIDForUsage, goalSessionKeyForSlot(slot)),
		InputOptions:     runtimectx.RuntimeInputOptionsForPurpose(roomRoundInputOptions(roundValue), "goal_continuation"),
		Client:           client,
		Mapper:           roomRoundMapperAdapter{mapper: mapper},
		IdleTimeout:      s.config.RuntimeRoundIdleTimeout(),
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
				fields := trace.BuildSDKMessageLogFieldsWithOptions(
					incoming,
					trace.SDKMessageLogOptions{
						IncludeStreamEvent:  s.config.MessageDebugStreamEvent,
						IncludeSnapshotData: true,
					},
				)
				if len(fields) == 0 {
					return
				}
				streamLogger.Debug("Room slot 收到 SDK 消息", fields...)
			}
		},
		SyncSessionID: func(sessionID string) error {
			return s.syncSlotSDKSessionID(slotCtx, slot, sessionID)
		},
		HandleDurableMessage: func(messageValue protocol.Message) error {
			messageRole := protocol.MessageRole(messageValue)
			slot.rememberSubagentTaskMessage(messageValue)
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
			// 剥离混合内容里的无回复标记，确保它不进入存储与公区。
			messageValue = roomdomain.StripNoReplyMarker(messageValue)
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
			for _, readyEvent := range slot.eventsReadyForEmission(event) {
				s.broadcastSharedEventWithTimeout(slotCtx, roundValue.SessionKey, roundValue.RoomID, readyEvent)
			}
			return nil
		},
	})
	if err != nil {
		if errors.Is(err, exec.ErrRoundInterrupted) {
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
	s.broadcastAgentRoundStatus(slotCtx, roundValue, slot, slot.getStatus())
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
		roundValue.RootRoundID,
		slot.AgentRoundID,
	))
	logger.Info("Room slot 结束",
		"status", slot.getStatus(),
		"result_subtype", strings.TrimSpace(result.ResultSubtype),
		"error_message", strings.TrimSpace(result.ErrorMessage),
	)
}
