// INPUT: Room round/slot、稳定 execution contract、trusted WorkBinding/ReviewBinding、Agent 配置、Goal context 与 runtime provider。
// OUTPUT: static/dynamic prompt 分层、producer/reviewer capability 绑定、真实 Agent slot lease、revision 绑定且换代安全的 runtime options/client。
// POS: Room slot 执行前不丢失 structured dispatch capability，并在连接前后复核身份的 runtime 装配边界。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	nexusRoomIDEnvName             = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName        = "NEXUS_ROOM_AGENT_ID"
)

type preparedSlotRuntime struct {
	options   agentclient.Options
	selection runtimeselectionsvc.Selection
	provider  string
}

type roomRuntimePrompt struct {
	// stable 是 execution contract、房间规则、技能和成员目录；这些变化才应使 prompt cache 前缀失效。
	stable string
	// dynamic 是 Agent runtime prompt；轮次与 Goal 上下文仍通过 user/contextual input 注入。
	dynamic string
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

func (s *Service) resolveReusableRoomSDKSessionID(
	ctx context.Context,
	logger *slog.Logger,
	workspacePath string,
	slot *activeRoomSlot,
	resumeID string,
) (string, error) {
	resumeID = strings.TrimSpace(resumeID)
	if resumeID == "" {
		return "", nil
	}
	history := s.history.ForOwner(slot.OwnerUserID)
	decision := sessionresumesvc.NewPolicy(history).CanResume(workspacePath, resumeID)
	if decision.Allowed {
		return resumeID, nil
	}
	if decision.Err != nil {
		logger.Warn("检查 Room SDK session transcript 失败，跳过过期 resume",
			"agent_id", slot.AgentID,
			"agent_round_id", slot.AgentRoundID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"room_session_id", slot.RoomSessionID,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		if clearErr := s.clearSlotSDKSessionID(ctx, slot); clearErr != nil {
			return "", clearErr
		}
		return "", nil
	}

	logger.Warn("Room SDK session transcript 不存在，跳过过期 resume",
		"agent_id", slot.AgentID,
		"agent_round_id", slot.AgentRoundID,
		"runtime_session_key", slot.RuntimeSessionKey,
		"room_session_id", slot.RoomSessionID,
		"workspace_path", workspacePath,
		"sdk_session_id", decision.SessionID,
		"reason", string(decision.Reason),
	)
	if err := s.clearSlotSDKSessionID(ctx, slot); err != nil {
		return "", err
	}
	return "", nil
}

func (e *slotExecution) prepareRuntimeClient() (runtimectx.Client, error) {
	if e.round == nil {
		return nil, errors.New("room round is required")
	}
	if err := requireGroupRoomContext(e.round.Context); err != nil {
		return nil, err
	}
	if err := workspacepkg.EnsureUserSkillLibrary(e.service.config, e.agent.OwnerUserID); err != nil {
		return nil, err
	}
	if err := workspacepkg.EnsureInitializedForAgent(e.service.config, *e.agent); err != nil {
		return nil, err
	}
	runtimeValue, err := e.prepareRuntime()
	if err != nil {
		return nil, err
	}
	client, err := e.connectRuntime(&runtimeValue)
	if err != nil {
		return nil, err
	}
	e.logger.Info("Room runtime 启动成功",
		append(roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot),
			"sdk_session_id", strings.TrimSpace(client.SessionID()),
		)...,
	)
	if sessionID := strings.TrimSpace(client.SessionID()); sessionID != "" {
		e.slot.ensureSDKSessionIdentityState().Set(sessionID)
	}
	return client, nil
}

func (e *slotExecution) prepareRuntime() (preparedSlotRuntime, error) {
	prompt, permissionMode, err := e.buildRuntimePrompt()
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	beginGoalUsageForSlot(e.slot)

	selection, err := e.service.resolveAgentRuntimeSelection(e.ctx, e.round, e.agent)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	e.emotionEnabled = selection.EmotionEnabled
	if err = e.service.agents.EnsureRuntimeVisionSettingsProjection(
		*e.agent,
		selection.VisionProvider,
		selection.VisionModel,
	); err != nil {
		return preparedSlotRuntime{}, err
	}
	runtimeSkillNames, err := workspacepkg.RuntimeSkillNamesForAgent(e.service.config, *e.agent)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	runtimeDisabledSkillNames, err := workspacepkg.RuntimeDisabledSkillNamesForAgent(
		e.service.config,
		*e.agent,
	)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	allowedTools, disallowedTools, snapshottedToolPolicy := roomRoundToolPolicy(e.round, e.agent)
	allowedTools = roomAllowedTools(allowedTools, e.round.Context.Room.PrivateMessagesEnabled)
	if !snapshottedToolPolicy {
		allowedTools = toolpolicy.WithManagedRuntimeAllowedTools(
			allowedTools,
			e.service.runtimeImagegenDefaultEnabled(e.ctx),
		)
	}
	disallowedTools = roomDisallowedTools(disallowedTools, e.round.Context.Room.PrivateMessagesEnabled)
	configurationRuntimeEnv := map[string]string(nil)
	if e.service.configurationRuntimeEnv != nil {
		configurationRuntimeEnv, err = e.service.configurationRuntimeEnv(
			e.runtimeBuilderContext(),
			e.agent,
			e.round.SessionKey,
			e.round.RootRoundID,
			roomCommandSourceContextType(e.round),
			e.round.RoomID,
		)
		if err != nil {
			return preparedSlotRuntime{}, err
		}
	}
	runtimeCommandEnv := map[string]string(nil)
	if e.service.runtimeCommandEnv != nil {
		runtimeCommandEnv, err = e.service.runtimeCommandEnv(
			e.runtimeBuilderContext(),
			e.runtimeCommandRoundContext(permissionMode),
		)
		if err != nil {
			return preparedSlotRuntime{}, err
		}
	}
	extraEnv := e.service.roomRuntimeEnv(e.round, e.slot)
	options, runtimeConfig, err := clientopts.BuildAgentClientOptionsWithConfig(e.ctx, e.service.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              e.agent.WorkspacePath,
		OwnerUserID:                e.agent.OwnerUserID,
		IsMainAgent:                e.agent.IsMain,
		RuntimeKind:                selection.RuntimeKind,
		Provider:                   selection.Provider,
		Model:                      selection.Model,
		VisionProvider:             selection.VisionProvider,
		VisionModel:                selection.VisionModel,
		PermissionMode:             permissionMode,
		PermissionHandler:          e.runtimePermissionHandler(),
		AllowedTools:               allowedTools,
		DisallowedTools:            disallowedTools,
		SkillIDs:                   runtimeSkillNames,
		DisabledSkillIDs:           runtimeDisabledSkillNames,
		SkillDirectories:           workspacepkg.SkillLibraryRoots(e.service.config, e.agent.OwnerUserID),
		SettingSources:             e.agent.Options.SettingSources,
		AppendSystemPrompt:         appendPromptSection(prompt.stable, prompt.dynamic),
		AppendSystemPromptStatic:   prompt.stable,
		AppendSystemPromptDynamic:  prompt.dynamic,
		ResumeSessionID:            e.slot.getSDKSessionID(),
		MaxThinkingTokens:          e.agent.Options.MaxThinkingTokens,
		MaxTurns:                   e.agent.Options.MaxTurns,
		MCPServers:                 e.runtimeMCPServers(permissionMode),
		AgentMCPServers:            e.agent.Options.MCPServers,
		ExtraEnv:                   extraEnv,
		ConfigurationEnv:           configurationRuntimeEnv,
		RuntimeCommandEnv:          runtimeCommandEnv,
		AgentSDKDiagnosticsEnabled: selection.AgentSDKDiagnosticsEnabled,
		ToolSearchEnabled:          selection.ToolSearchEnabled,
		WebSearch:                  selection.WebSearch,
		RuntimeIsolationMode:       e.service.config.RuntimeIsolationMode,
		RuntimeLauncherPath:        e.service.config.RuntimeLauncherPath,
	})
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	if runtimeConfig != nil {
		e.slot.setContextWindow(runtimeConfig.ContextWindow)
	}

	e.slot.setRuntimeKind(string(options.Runtime.Kind))
	options = e.applyRuntimeHooks(options)
	runtimeProvider := clientopts.ResolvedRuntimeProvider(selection.Provider, options)
	return preparedSlotRuntime{options: options, selection: selection, provider: runtimeProvider}, nil
}

func (e *slotExecution) buildRuntimePrompt() (roomRuntimePrompt, sdkpermission.Mode, error) {
	dynamicPrompt, err := e.service.agents.BuildRuntimePrompt(e.ctx, e.agent)
	if err != nil {
		return roomRuntimePrompt{}, "", err
	}
	stablePrompt := appendPromptSection(
		orchestration.StablePrompt(),
		roomdomain.BuildSystemPrompt(e.round.Context.Room.PrivateMessagesEnabled),
	)
	roomSkillPrompt, err := e.service.rooms.BuildRoomSkillPrompt(e.ctx, e.round.Context.Room.SkillNames)
	if err != nil {
		return roomRuntimePrompt{}, "", err
	}
	stablePrompt = appendPromptSection(stablePrompt, roomSkillPrompt)
	stablePrompt = appendPromptSection(stablePrompt, roomdomain.BuildMemberDirectoryPrompt(e.agentNameByID))

	sessionSettings := protocol.SessionRuntimeSettingsFromOptions(
		roomAgentSessionOptions(e.round, e.agent.AgentID),
	)
	permissionMode := runtimepermission.NormalizeMode(
		sdkpermission.Mode(e.agent.Options.PermissionMode),
	)
	if sessionSettings.PermissionMode != "" {
		permissionMode = runtimepermission.NormalizeMode(
			sdkpermission.Mode(sessionSettings.PermissionMode),
		)
	}
	if e.round.PermissionMode != "" {
		permissionMode = runtimepermission.NormalizeMode(e.round.PermissionMode)
	}
	e.slot.setGoalRuntimeIgnored(goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode)))
	// The session's ambient Goal is not a round capability. Start unbound and
	// admit only an explicit continuation or an exact Goal-bound Work/Review
	// Execution. A successful create_goal can bind this slot later.
	e.slot.setGoalContext("")
	e.slot.setGoalBinding(strings.TrimSpace(e.round.SessionKey), "")
	if !e.slot.goalRuntimeIgnored() {
		explicitGoalID := strings.TrimSpace(e.round.GoalID)
		explicitRevision := e.round.GoalObjectiveRevision
		if e.slot.WorkBinding != nil || e.slot.ReviewBinding != nil {
			goalContext, authority, granted, bindingErr := e.service.resolveExecutionGoalMutationAuthority(
				e.ctx,
				e.orchestrationActor(),
				e.round.RootRoundID,
			)
			if bindingErr != nil {
				return roomRuntimePrompt{}, "", bindingErr
			}
			if granted {
				if !e.slot.grantGoalMutationAuthority(authority) {
					return roomRuntimePrompt{}, "", goalsvc.ErrGoalRevisionStale
				}
				e.slot.setGoalContext(goalContext)
			}
		} else if e.round.Internal && explicitGoalID != "" && explicitRevision > 0 {
			goalContext, currentGoalID, currentRevision, ok := e.service.goalRuntimeContext(
				e.ctx,
				strings.TrimSpace(e.round.SessionKey),
			)
			if !ok || currentGoalID != explicitGoalID || currentRevision != explicitRevision {
				return roomRuntimePrompt{}, "", goalsvc.ErrGoalRevisionStale
			}
			if !e.slot.grantGoalMutationAuthority(roomGoalMutationAuthority{
				SessionKey:        strings.TrimSpace(e.round.SessionKey),
				GoalID:            explicitGoalID,
				ObjectiveRevision: explicitRevision,
				ExecutionID: firstNonEmptyString(
					executionIDFromRoomBindings(
						e.slot.WorkBinding,
						e.slot.ReviewBinding,
					),
					e.round.ExecutionID,
				),
				RootRoundID: strings.TrimSpace(e.round.RootRoundID),
				Source:      roomGoalAuthorityExplicitRound,
			}) {
				return roomRuntimePrompt{}, "", goalsvc.ErrGoalRevisionStale
			}
			e.slot.setGoalContext(goalContext)
		}
	}
	if override := strings.TrimSpace(e.round.GoalContext); e.round.Internal && override != "" {
		e.slot.setGoalContext(override)
	}
	return roomRuntimePrompt{stable: stablePrompt, dynamic: dynamicPrompt}, permissionMode, nil
}

func (e *slotExecution) runtimeMCPServers(permissionMode sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	goalAuthority := e.slot.ensureGoalAuthorityState()
	responsibilityAuthority := e.ensureResponsibilityAuthorityState()
	if responsibilityAuthority != nil {
		responsibilityAuthority.SeedExecution(firstNonEmptyString(
			executionIDFromRoomBindings(e.slot.WorkBinding, e.slot.ReviewBinding),
			e.round.ExecutionID,
		))
	}
	var servers map[string]sdkmcp.ServerConfig
	if e.service.mcpServers != nil {
		sourceContextType := roomCommandSourceContextType(e.round)
		mcpContext := runtimectx.WithResponsibilityAuthorityState(
			runtimectx.WithGoalAuthorityState(
				e.runtimeBuilderContext(),
				goalAuthority,
			),
			responsibilityAuthority,
		)
		mcpContext = runtimectx.WithEnabledConnectorIDs(
			mcpContext,
			protocol.EffectiveSessionConnectorIDs(
				e.agent.Options.ConnectorIDs,
				roomAgentSessionOptions(e.round, e.agent.AgentID),
			),
		)
		servers = e.service.mcpServers(
			mcpContext,
			e.agent,
			e.round.SessionKey,
			e.round.RootRoundID,
			sourceContextType,
			e.round.RoomID,
			roomSourceContextLabel(e.round),
			e.slot.ensureGoalObjectiveRevision(0),
			permissionMode,
		)
	}
	return servers
}

func (e *slotExecution) runtimeCommandRoundContext(permissionMode sdkpermission.Mode) runtimecommand.RoundContext {
	goalAuthority := e.slot.ensureGoalAuthorityState()
	responsibilityAuthority := e.ensureResponsibilityAuthorityState()
	if responsibilityAuthority != nil {
		responsibilityAuthority.SeedExecution(firstNonEmptyString(
			executionIDFromRoomBindings(e.slot.WorkBinding, e.slot.ReviewBinding),
			e.round.ExecutionID,
		))
	}
	commandContext := runtimectx.RuntimeCommandContext{
		Agent:             e.agent,
		ScopeSessionKey:   e.round.SessionKey,
		RuntimeSessionKey: e.slot.RuntimeSessionKey,
		ExecutionID: firstNonEmptyString(
			executionIDFromRoomBindings(
				e.slot.WorkBinding,
				e.slot.ReviewBinding,
			),
			e.round.ExecutionID,
		),
		WorkBinding:             cloneExecutionWorkBinding(e.slot.WorkBinding),
		WorkBindingState:        e.ensureWorkBindingState(),
		ReviewBinding:           cloneExecutionReviewBinding(e.slot.ReviewBinding),
		CoordinatorAgentID:      strings.TrimSpace(e.round.CoordinatorAgentID),
		RootRoundID:             e.round.RootRoundID,
		AgentRoundID:            e.slot.AgentRoundID,
		SourceContextType:       "room",
		SourceContextID:         e.round.RoomID,
		SourceContextLabel:      roomSourceContextLabel(e.round),
		RoomID:                  e.round.RoomID,
		ConversationID:          e.round.ConversationID,
		PermissionMode:          permissionMode,
		GoalAuthority:           goalAuthority,
		ResponsibilityAuthority: responsibilityAuthority,
		SDKSessionIdentity:      e.slot.ensureSDKSessionIdentityState(),
		AutomationRun:           cloneAutomationRunContext(e.round.AutomationRun),
	}
	return runtimecommand.RoundContext{
		SessionKey: e.round.SessionKey, RoundID: e.round.RootRoundID,
		SourceContextType: roomCommandSourceContextType(e.round),
		SourceContextID:   e.round.RoomID, SourceContextLabel: roomSourceContextLabel(e.round),
		CommandContext: commandContext, Receipts: e.slot.ensureCommandReceiptState(),
		Resources: e.slot.ensureCommandResources(),
	}
}

func (e *slotExecution) runtimeBuilderContext() context.Context {
	return runtimectx.WithRuntimeRoundLease(
		e.ctx,
		e.slot.RuntimeSessionKey,
		e.slot.AgentRoundID,
	)
}

func roomCommandSourceContextType(round *activeRoomRound) string {
	if round == nil {
		return "room_untrusted"
	}
	if round.trustedQueuedConfigurationContext &&
		strings.TrimSpace(round.ExecutionOrigin) == "queue" {
		return "room"
	}
	switch {
	case strings.TrimSpace(round.ExecutionOrigin) != "":
		return "room_" + strings.ToLower(strings.TrimSpace(round.ExecutionOrigin))
	case round.Internal:
		return "room_internal"
	case !round.TrustedConfigurationContext:
		return "room_untrusted"
	default:
		return "room"
	}
}

func (e *slotExecution) runtimePermissionHandler() sdkpermission.Handler {
	handler := e.round.PermissionHandler
	if handler == nil {
		handler = func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
			return e.service.permission.RequestPermission(ctx, e.slot.RuntimeSessionKey, request)
		}
	}
	allowedTools, disallowedTools, _ := roomRoundToolPolicy(e.round, e.agent)
	handler = withRoomPermissionPolicy(
		handler,
		e.round.Context.Room.PrivateMessagesEnabled,
		allowedTools,
		disallowedTools,
	)
	handler = toolpolicy.WithManagedRuntimeAutoApproval(handler)
	handler = toolpolicy.WithNexusRuntimeCLIAutoApproval(handler)
	handler = toolpolicy.WithMalformedInputDeny(handler)
	return toolpolicy.WithNexusControlPlaneDeny(handler, !e.agent.IsMain)
}

func (e *slotExecution) applyRuntimeHooks(options agentclient.Options) agentclient.Options {
	options = e.service.runtime.WithGuidanceHook(options, e.slot.RuntimeSessionKey)
	options = e.service.runtime.WithSubagentAdmissionHooks(options, e.slot.RuntimeSessionKey)
	if goalSessionKey := goalSessionKeyForSlot(e.slot); goalSessionKey != "" && goalSessionKey != e.slot.RuntimeSessionKey {
		options = e.service.runtime.WithGuidanceHook(options, goalSessionKey)
	}
	options = runtimectx.WithPostToolUseGuidanceHook(options, e.service.roomSlotGuidanceHook(e.round, e.slot, workspacestore.InputQueueLocation{
		OwnerUserID:    e.round.OwnerUserID,
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  e.agent.WorkspacePath,
		SessionKey:     e.slot.RuntimeSessionKey,
		RoomID:         e.round.RoomID,
		ConversationID: e.round.ConversationID,
	}))
	return withRoomRuntimeDiagnosticsLogger(options, e.logger.With("agent_id", e.slot.AgentID, "agent_round_id", e.slot.AgentRoundID))
}

func (e *slotExecution) connectRuntime(runtimeValue *preparedSlotRuntime) (runtimectx.Client, error) {
	startup, err := e.service.runtime.BeginClientStartup(e.ctx, e.slot.RuntimeSessionKey, e.round.OwnerUserID)
	if err != nil {
		return nil, err
	}
	defer startup.Close()
	currentResumeID, err := e.reloadSlotSDKSessionID()
	if err != nil {
		return nil, err
	}
	resumeID, err := e.service.resolveReusableRoomSDKSessionID(
		e.ctx,
		e.logger,
		e.agent.WorkspacePath,
		e.slot,
		currentResumeID,
	)
	if err != nil {
		return nil, err
	}
	runtimeValue.options.Session.ResumeID = resumeID

	client, err := e.connectRuntimeOnce(startup, *runtimeValue)
	if err != nil && strings.TrimSpace(runtimeValue.options.Session.ResumeID) != "" && runtimectx.IsRuntimeTransportClosedError(err) {
		e.logger.Warn("Room SDK session resume 失效，清除后重试",
			append(roomRuntimeConnectFailureLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot, err),
				"sdk_session_id", strings.TrimSpace(runtimeValue.options.Session.ResumeID),
			)...,
		)
		retired, closeErr := retireRoomRuntimeClient(startup)
		if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			e.logger.Warn("清理失效 resume 的 Room runtime 返回错误",
				"startup_err", err,
				"cleanup_err", closeErr,
			)
		}
		if retired {
			if clearErr := e.service.clearSlotSDKSessionID(e.ctx, e.slot); clearErr != nil {
				return nil, clearErr
			}
			runtimeValue.options.Session.ResumeID = ""
			if !errors.Is(closeErr, context.Canceled) && !errors.Is(closeErr, context.DeadlineExceeded) {
				client, err = e.connectRuntimeOnce(startup, *runtimeValue)
			}
		}
	}
	if err == nil {
		return client, nil
	}
	if _, closeErr := retireRoomRuntimeClient(startup); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
		e.logger.Warn("清理启动失败的 Room runtime 返回错误", "err", closeErr)
	}
	e.logger.Error("Room runtime 启动失败", roomRuntimeConnectFailureLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot, err)...)
	return nil, err
}

func (e *slotExecution) reloadSlotSDKSessionID() (string, error) {
	cached := e.slot.getSDKSessionID()
	if e.service.rooms == nil || strings.TrimSpace(e.round.ConversationID) == "" {
		return cached, nil
	}
	contextValue, err := e.service.rooms.GetConversationContext(e.ctx, e.round.ConversationID)
	if err != nil {
		return "", err
	}
	if contextValue == nil {
		return cached, nil
	}
	roomSessionID := strings.TrimSpace(e.slot.RoomSessionID)
	for _, sessionRecord := range contextValue.Sessions {
		if strings.TrimSpace(sessionRecord.ID) != roomSessionID {
			continue
		}
		resumeID := strings.TrimSpace(sessionRecord.SDKSessionID)
		if resumeID == "" {
			e.slot.clearSDKSessionID()
		} else {
			e.slot.setSDKSessionID(resumeID)
		}
		return resumeID, nil
	}
	return cached, nil
}

func retireRoomRuntimeClient(startup *runtimectx.ClientStartup) (bool, error) {
	closeCtx, cancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	return startup.RetireCurrent(closeCtx)
}

func (e *slotExecution) connectRuntimeOnce(
	startup *runtimectx.ClientStartup,
	runtimeValue preparedSlotRuntime,
) (runtimectx.Client, error) {
	e.logger.Info("准备启动 Room runtime",
		roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot)...,
	)
	previousClient := e.service.runtime.SessionClient(e.slot.RuntimeSessionKey)
	hadWarmSession := e.service.runtime.HasSession(e.slot.RuntimeSessionKey)
	client, err := startup.GetOrCreateWithFactory(
		e.ctx,
		runtimeValue.options,
		e.service.factory,
	)
	if err != nil {
		return client, err
	}
	e.slot.setRuntimeKind(string(e.service.runtime.RuntimeKind(e.slot.RuntimeSessionKey)))
	e.slot.setClient(client)
	if err = startup.Connect(e.ctx); err != nil {
		return client, err
	}
	reusedWarmSession := hadWarmSession && previousClient == client
	e.slot.setContextColdStart(roomContextColdStart(runtimeValue.options.Session.ResumeID, reusedWarmSession))
	return client, nil
}

// roomContextColdStart 只把首次创建且没有可用 resume 的 slot 视为冷启动。
// Manager 中仍存活的 client 已经保有 Room cursor，不应重复灌入完整公区历史。
func roomContextColdStart(resumeID string, hadWarmSession bool) bool {
	return strings.TrimSpace(resumeID) == "" && !hadWarmSession
}

func (s *Service) roomRuntimeEnv(roundValue *activeRoomRound, slot *activeRoomSlot) map[string]string {
	if roundValue == nil || slot == nil {
		return nil
	}
	env := map[string]string{
		nexusRoomIDEnvName:             strings.TrimSpace(roundValue.RoomID),
		nexusRoomConversationIDEnvName: strings.TrimSpace(roundValue.ConversationID),
		nexusRoomAgentIDEnvName:        strings.TrimSpace(slot.AgentID),
	}
	return env
}

type imagegenDefaultResolver interface {
	ResolveImageConfig(context.Context, string) (*providercfg.ImageConfig, error)
}

func (s *Service) runtimeImagegenDefaultEnabled(ctx context.Context) bool {
	resolver, ok := s.providers.(imagegenDefaultResolver)
	if !ok || resolver == nil {
		return false
	}
	_, err := resolver.ResolveImageConfig(ctx, "")
	return err == nil
}

func (s *Service) resolveAgentRuntimeSelection(
	ctx context.Context,
	roundValue *activeRoomRound,
	agentValue *protocol.Agent,
) (runtimeselectionsvc.Selection, error) {
	ownerUserIDs := []string(nil)
	if roundValue != nil {
		ownerUserIDs = append(ownerUserIDs, roundValue.OwnerUserID)
	}
	return runtimeselectionsvc.NewServiceWithRuntimeConfigResolver(s.prefs, s.providers).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:          agentValue,
		OwnerUserIDs:   ownerUserIDs,
		SessionOptions: roomAgentSessionOptions(roundValue, agentValue.AgentID),
	})
}

func roomAgentSessionOptions(
	roundValue *activeRoomRound,
	agentID string,
) map[string]any {
	if roundValue == nil || roundValue.Context == nil {
		return nil
	}
	return roomSessionOptionsFromContext(roundValue.Context, agentID)
}

func roomSessionOptionsFromContext(
	contextValue *protocol.ConversationContextAggregate,
	agentID string,
) map[string]any {
	if contextValue == nil {
		return nil
	}
	agentID = strings.TrimSpace(agentID)
	for _, sessionValue := range contextValue.Sessions {
		if sessionValue.AgentID == agentID && sessionValue.IsPrimary {
			return sessionValue.Options
		}
	}
	return nil
}

func roomRuntimeStartupLogFields(
	options agentclient.Options,
	runtimeSelection runtimeselectionsvc.Selection,
	runtimeProvider string,
	slot *activeRoomSlot,
) []any {
	return append(clientopts.RuntimeStartupLogFields(options),
		"agent_id", slot.AgentID,
		"agent_round_id", slot.AgentRoundID,
		"runtime_session_key", slot.RuntimeSessionKey,
		"requested_runtime_kind", strings.TrimSpace(runtimeSelection.RuntimeKind),
		"requested_provider", strings.TrimSpace(runtimeSelection.Provider),
		"requested_model", strings.TrimSpace(runtimeSelection.Model),
		"runtime_provider", runtimeProvider,
	)
}

func roomRuntimeConnectFailureLogFields(
	options agentclient.Options,
	runtimeSelection runtimeselectionsvc.Selection,
	runtimeProvider string,
	slot *activeRoomSlot,
	err error,
) []any {
	return append(roomRuntimeStartupLogFields(options, runtimeSelection, runtimeProvider, slot),
		"stage", "connect",
		"err", err,
		"error_type", fmt.Sprintf("%T", err),
		"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
	)
}

func withRoomRuntimeDiagnosticsLogger(options agentclient.Options, logger *slog.Logger) agentclient.Options {
	previousStderr := options.Callbacks.Stderr
	options.Callbacks.Stderr = func(line string) {
		normalizedLine := runtimectx.NormalizeRuntimeStderrLine(line)
		if previousStderr != nil {
			previousStderr(normalizedLine)
		}
		logger.Debug("Agent SDK stderr", "stderr", normalizedLine)
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
