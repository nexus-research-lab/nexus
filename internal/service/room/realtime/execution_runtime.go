// INPUT: Room round/slot、成员 Session 本机目录、稳定 execution contract、trusted WorkBinding/ReviewBinding、Agent 配置、Goal context 与 runtime provider。
// OUTPUT: static/dynamic prompt 分层、本机目录授权、producer/reviewer capability 绑定、固定父 round Subagent control、真实 Agent slot lease、factory 前 desktop sandbox lease 身份绑定与创建后所有权交接、工具面换代且 revision 绑定的 runtime options/client。
// POS: Room slot 执行前不丢失 structured dispatch capability，并在连接前后复核身份、失败回收 sandbox lease 的 runtime 装配边界。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	nexusRoomIDEnvName             = "NEXUS_ROOM_ID"
	nexusRoomConversationIDEnvName = "NEXUS_ROOM_CONVERSATION_ID"
	nexusRoomAgentIDEnvName        = "NEXUS_ROOM_AGENT_ID"
)

type preparedSlotRuntime struct {
	options                agentclient.Options
	selection              runtimeselectionsvc.Selection
	provider               string
	toolSurfaceFingerprint string
	toolSurfaceComplete    bool
	forkLegacyToolSurface  bool
	scratchLease           *runtimectx.SandboxResourceLease
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
	history := s.History.ForOwner(slot.OwnerUserID)
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
	runtimeValue, err := e.prepareRuntime()
	if err != nil {
		return nil, err
	}
	client, err := e.connectRuntime(&runtimeValue)
	if err != nil {
		// connectRuntime 在交给 RuntimeConnect 前失败时，scratch 仍归这里释放。
		if runtimeValue.scratchLease != nil {
			_ = runtimeValue.scratchLease.Release()
		}
		return nil, err
	}
	e.logger.Info("Room runtime 启动成功",
		append(roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot),
			"sdk_session_id", strings.TrimSpace(client.SessionID()),
		)...,
	)
	if sessionID := strings.TrimSpace(client.SessionID()); sessionID != "" {
		e.slot.ensureSDKSessionIdentityState().Set(sessionID)
	} else if e.forkSourceSessionID != "" {
		e.slot.ensureSDKSessionIdentityState().Set("")
	}
	return client, nil
}

func (e *slotExecution) prepareRuntime() (preparedSlotRuntime, error) {
	// Skill 库与 Agent workspace 初始化必须早于 prompt 构建，prompt 会读取其中的文件。
	skillIDs, disabledSkillIDs, err := e.service.AgentRuntimeSkills(*e.agent, nil)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	prompt, permissionMode, err := e.buildRuntimePrompt()
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	beginGoalUsageForSlot(e.slot)

	sessionOptions := roomAgentSessionOptions(e.round, e.agent.AgentID)
	selection, err := e.service.ResolveAgentRuntimeSelection(e.ctx, e.agent, sessionOptions, e.round.OwnerUserID)
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	e.emotionEnabled = selection.EmotionEnabled
	if err = e.service.Agents.EnsureRuntimeVisionSettingsProjection(
		*e.agent,
		selection.VisionProvider,
		selection.VisionModel,
	); err != nil {
		return preparedSlotRuntime{}, err
	}
	allowedTools, disallowedTools, snapshottedToolPolicy := roomRoundToolPolicy(e.round, e.agent)
	allowedTools = roomAllowedTools(allowedTools, e.round.Context.Room.PrivateMessagesEnabled)
	if !snapshottedToolPolicy {
		allowedTools = toolpolicy.WithManagedRuntimeAllowedTools(
			allowedTools,
			e.service.RuntimeImagegenDefaultEnabled(e.ctx),
		)
	}
	disallowedTools = roomDisallowedTools(disallowedTools, e.round.Context.Room.PrivateMessagesEnabled)
	configurationRuntimeEnv := map[string]string(nil)
	if e.service.ConfigurationRuntimeEnv != nil {
		configurationRuntimeEnv, err = e.service.ConfigurationRuntimeEnv(
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
	mcpContext := e.runtimeMCPContext()
	mcpServers := e.runtimeMCPServers(mcpContext, permissionMode)
	if e.service.NexusMCP != nil {
		mcpServers, err = e.service.MergeNexusMCPServers(mcpContext, mcpServers, e.runtimeCommandRoundContext(permissionMode))
		if err != nil {
			return preparedSlotRuntime{}, err
		}
	}
	scratchLease, err := e.service.AcquireRuntimeScratch(e.ctx, selection, permissionMode, e.scratchInput())
	if err != nil {
		return preparedSlotRuntime{}, err
	}
	scratchLeaseOwned := false
	defer func() {
		if scratchLease != nil && !scratchLeaseOwned {
			_ = scratchLease.Release()
		}
	}()
	options, runtimeConfig, err := e.service.BuildAgentRuntimeOptions(e.ctx, runtimehost.AgentRuntimeOptionsInput{
		Agent:                 e.agent,
		Selection:             selection,
		PermissionMode:        permissionMode,
		PermissionHandler:     e.runtimePermissionHandler(),
		AllowedTools:          allowedTools,
		DisallowedTools:       disallowedTools,
		SkillIDs:              skillIDs,
		DisabledSkillIDs:      disabledSkillIDs,
		AdditionalDirectories: protocol.SessionAdditionalDirectoriesFromOptions(sessionOptions),
		StablePrompt:          prompt.stable,
		DynamicPrompt:         prompt.dynamic,
		ResumeSessionID:       e.slot.getSDKSessionID(),
		MCPServers:            mcpServers,
		ExtraEnv:              e.service.roomRuntimeEnv(e.round, e.slot),
		ConfigurationEnv:      configurationRuntimeEnv,
		Scratch:               scratchLease,
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
	toolSurfaceFingerprint, toolSurfaceComplete, err := runtimectx.ModelToolSurfaceFingerprint(e.ctx, options)
	if err != nil {
		return preparedSlotRuntime{}, fmt.Errorf("计算 Room runtime 工具面指纹: %w", err)
	}
	scratchLeaseOwned = true
	return preparedSlotRuntime{
		options:                options,
		selection:              selection,
		provider:               runtimeProvider,
		toolSurfaceFingerprint: toolSurfaceFingerprint,
		toolSurfaceComplete:    toolSurfaceComplete,
		scratchLease:           scratchLease,
		forkLegacyToolSurface: len(protocol.EffectiveSessionConnectorIDs(
			e.agent.Options.ConnectorIDs,
			sessionOptions,
		)) > 0,
	}, nil
}

func (e *slotExecution) scratchInput() runtimectx.SandboxResourceInput {
	return runtimectx.SandboxResourceInput{
		OwnerUserID: e.agent.OwnerUserID,
		SessionKey:  e.slot.RuntimeSessionKey,
		RoundID:     e.round.RootRoundID,
	}
}

func (e *slotExecution) buildRuntimePrompt() (roomRuntimePrompt, sdkpermission.Mode, error) {
	dynamicPrompt, err := e.service.Agents.BuildRuntimePrompt(e.ctx, e.agent)
	if err != nil {
		return roomRuntimePrompt{}, "", err
	}
	stablePrompt := runtimehost.JoinPromptSections(
		orchestration.StablePrompt(),
		roomdomain.BuildSystemPrompt(e.round.Context.Room.PrivateMessagesEnabled),
	)
	roomSkillPrompt, err := e.service.rooms.BuildRoomSkillPrompt(e.ctx, e.round.Context.Room.SkillNames)
	if err != nil {
		return roomRuntimePrompt{}, "", err
	}
	stablePrompt = runtimehost.JoinPromptSections(stablePrompt, roomSkillPrompt)
	directory := e.agentNameByID
	if e.round.ExecutionOrigin == "relay" && len(e.round.PublicAgentDirectory) > 0 {
		directory = e.round.PublicAgentDirectory
	}
	stablePrompt = runtimehost.JoinPromptSections(stablePrompt, roomdomain.BuildMemberDirectoryPrompt(directory))

	sessionSettings := protocol.SessionRuntimeSettingsFromOptions(
		roomAgentSessionOptions(e.round, e.agent.AgentID),
	)
	sessionKey := strings.TrimSpace(e.round.SessionKey)
	permissionMode := runtimepermission.NormalizeMode(
		sdkpermission.Mode(e.agent.Options.PermissionMode),
	)
	// 在线输入不继承 Agent 全局放行；仅允许主人显式设置该执行 Session 的权限。
	if e.round.ExecutionOrigin == "relay" {
		permissionMode = sdkpermission.ModeDefault
	}
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
	e.slot.setGoalBinding(sessionKey, "")
	if !e.slot.goalRuntimeIgnored() {
		explicitGoalID := e.round.GoalID
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
				sessionKey,
			)
			if !ok || currentGoalID != explicitGoalID || currentRevision != explicitRevision {
				return roomRuntimePrompt{}, "", goalsvc.ErrGoalRevisionStale
			}
			if !e.slot.grantGoalMutationAuthority(roomGoalMutationAuthority{
				SessionKey:        sessionKey,
				GoalID:            explicitGoalID,
				ObjectiveRevision: explicitRevision,
				ExecutionID: textutil.FirstNonEmpty(
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
	if override := e.round.GoalContext; e.round.Internal && override != "" {
		e.slot.setGoalContext(override)
	}
	if e.service.externalPrompt != nil {
		prompt, err := e.service.externalPrompt(e.ctx, roomRootRoundID(e.round), e.slot.AgentID, e.slot.RuntimeSessionKey)
		if err != nil {
			return roomRuntimePrompt{}, "", err
		}
		dynamicPrompt = runtimehost.JoinPromptSections(dynamicPrompt, prompt)
	}
	return roomRuntimePrompt{stable: stablePrompt, dynamic: dynamicPrompt}, permissionMode, nil
}

func (e *slotExecution) runtimeMCPContext() context.Context {
	goalAuthority := e.slot.ensureGoalAuthorityState()
	responsibilityAuthority := e.ensureResponsibilityAuthorityState()
	if responsibilityAuthority != nil {
		responsibilityAuthority.SeedExecution(textutil.FirstNonEmpty(
			executionIDFromRoomBindings(e.slot.WorkBinding, e.slot.ReviewBinding),
			e.round.ExecutionID,
		))
	}
	mcpContext := runtimectx.WithResponsibilityAuthorityState(
		runtimectx.WithGoalAuthorityState(
			e.runtimeBuilderContext(),
			goalAuthority,
		),
		responsibilityAuthority,
	)
	return runtimectx.WithEnabledConnectorIDs(
		mcpContext,
		protocol.EffectiveSessionConnectorIDs(
			e.agent.Options.ConnectorIDs,
			roomAgentSessionOptions(e.round, e.agent.AgentID),
		),
	)
}

func (e *slotExecution) runtimeMCPServers(
	ctx context.Context,
	permissionMode sdkpermission.Mode,
) map[string]sdkmcp.ServerConfig {
	if e.service.MCPServers == nil {
		return nil
	}
	return e.service.MCPServers(
		ctx,
		e.agent,
		e.round.SessionKey,
		e.round.RootRoundID,
		roomCommandSourceContextType(e.round),
		e.round.RoomID,
		roomSourceContextLabel(e.round),
		e.slot.ensureGoalObjectiveRevision(0),
		permissionMode,
	)
}

func (e *slotExecution) runtimeCommandRoundContext(permissionMode sdkpermission.Mode) nexusmcp.RoundContext {
	goalAuthority := e.slot.ensureGoalAuthorityState()
	responsibilityAuthority := e.ensureResponsibilityAuthorityState()
	if responsibilityAuthority != nil {
		responsibilityAuthority.SeedExecution(textutil.FirstNonEmpty(
			executionIDFromRoomBindings(e.slot.WorkBinding, e.slot.ReviewBinding),
			e.round.ExecutionID,
		))
	}
	commandContext := runtimectx.RuntimeCommandContext{
		Agent:             e.agent,
		ScopeSessionKey:   e.round.SessionKey,
		RuntimeSessionKey: e.slot.RuntimeSessionKey,
		ExecutionID: textutil.FirstNonEmpty(
			executionIDFromRoomBindings(
				e.slot.WorkBinding,
				e.slot.ReviewBinding,
			),
			e.round.ExecutionID,
		),
		WorkBinding:             e.slot.WorkBinding.Clone(),
		WorkBindingState:        e.ensureWorkBindingState(),
		ReviewBinding:           e.slot.ReviewBinding.Clone(),
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
		AutomationRun:           e.round.AutomationRun.NormalizedCopy(),
	}
	return nexusmcp.RoundContext{
		SessionKey: e.round.SessionKey, RoundID: e.round.RootRoundID,
		SubagentControl:   e.service.Runtime.BindSubagentControl(e.slot.RuntimeSessionKey, e.slot.AgentRoundID),
		SourceContextType: roomCommandSourceContextType(e.round),
		SourceContextID:   e.round.RoomID, SourceContextLabel: roomSourceContextLabel(e.round),
		CommandContext: commandContext, CommandReceipts: e.slot.ensureCommandReceiptState(),
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
	executionOrigin := round.ExecutionOrigin
	if round.trustedQueuedConfigurationContext && executionOrigin == "queue" {
		return "room"
	}
	switch {
	case executionOrigin != "":
		return "room_" + strings.ToLower(executionOrigin)
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
			if e.service.externalPermission != nil {
				external, err := e.service.externalPermission(ctx, roomRootRoundID(e.round), e.slot.AgentID, e.slot.RuntimeSessionKey)
				if err != nil {
					return sdkpermission.Deny("外部会话权限路由不可用", false), err
				}
				if external != nil {
					return external(ctx, request)
				}
			}
			return e.service.Permission.RequestPermission(ctx, e.slot.RuntimeSessionKey, request)
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
	handler = toolpolicy.WithMalformedInputDeny(handler)
	return toolpolicy.WithNexusControlPlaneDeny(handler, !e.agent.IsMain)
}

func (e *slotExecution) applyRuntimeHooks(options agentclient.Options) agentclient.Options {
	return e.service.WithAgentRuntimeHooks(options, e.slot.RuntimeSessionKey,
		e.logger.With("agent_id", e.slot.AgentID, "agent_round_id", e.slot.AgentRoundID),
		func(options agentclient.Options) agentclient.Options {
			if goalSessionKey := goalSessionKeyForSlot(e.slot); goalSessionKey != "" && goalSessionKey != e.slot.RuntimeSessionKey {
				options = e.service.Runtime.WithGuidanceHook(options, goalSessionKey)
			}
			return runtimectx.WithPostToolUseGuidanceHook(options, e.service.roomSlotGuidanceHook(e.round, e.slot, workspacestore.InputQueueLocation{
				OwnerUserID:    e.round.OwnerUserID,
				Scope:          protocol.InputQueueScopeRoom,
				WorkspacePath:  e.agent.WorkspacePath,
				SessionKey:     e.slot.RuntimeSessionKey,
				RoomID:         e.round.RoomID,
				ConversationID: e.round.ConversationID,
			}))
		},
	)
}

func (e *slotExecution) connectRuntime(runtimeValue *preparedSlotRuntime) (runtimectx.Client, error) {
	startup, err := e.service.Runtime.BeginClientStartup(e.ctx, e.slot.RuntimeSessionKey, e.round.OwnerUserID)
	if err != nil {
		return nil, err
	}
	defer startup.Close()
	currentResumeID, storedToolSurface, err := e.reloadSlotRuntimeIdentity()
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
	toolSurfaceFork := resumeID != "" && sessionresumesvc.RequiresToolSurfaceFork(
		storedToolSurface,
		runtimeValue.toolSurfaceFingerprint,
		runtimeValue.forkLegacyToolSurface,
	)
	if toolSurfaceFork && !runtimeValue.toolSurfaceComplete {
		return nil, errors.New("Room runtime 工具面检查不完整，无法安全 fork 旧 SDK session")
	}
	runtimeValue.options.Session.ResumeID = resumeID
	runtimeValue.options.Session.Fork = toolSurfaceFork
	e.toolSurfaceFingerprint = strings.TrimSpace(runtimeValue.toolSurfaceFingerprint)
	e.forkSourceSessionID = ""
	e.runtimeIdentityCommitted = resumeID != "" &&
		storedToolSurface == e.toolSurfaceFingerprint &&
		!toolSurfaceFork
	if toolSurfaceFork {
		retired, retireErr := runtimehost.RetireExistingRuntimeClient(e.ctx, startup)
		if retireErr != nil && !runtimectx.IsRuntimeTransportClosedError(retireErr) {
			return nil, fmt.Errorf("换代 Room runtime 工具面: %w", retireErr)
		}
		e.forkSourceSessionID = resumeID
		e.logger.Info("Room Session 工具面变化，从旧 transcript fork 新 SDK session",
			"runtime_session_key", e.slot.RuntimeSessionKey,
			"sdk_session_id", resumeID,
			"retired_warm_client", retired,
		)
	}

	// RuntimeConnect 接管 scratch：成功时交给 runtime，失败时由它释放。
	scratch := runtimeValue.scratchLease
	runtimeValue.scratchLease = nil
	client, err := runtimehost.RuntimeConnect{
		Startup:      startup,
		Options:      &runtimeValue.options,
		Scratch:      scratch,
		ScratchInput: e.scratchInput(),
		Logger:       e.logger.With(roomRuntimeStartupLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot)...),
		Attempt:      e.connectRuntimeOnce,
		ClearResume: func() error {
			if err := e.service.clearSlotSDKSessionID(e.ctx, e.slot); err != nil {
				return err
			}
			e.forkSourceSessionID = ""
			e.runtimeIdentityCommitted = false
			return nil
		},
	}.Run(e.ctx)
	if err != nil {
		e.logger.Error("Room runtime 启动失败", roomRuntimeConnectFailureLogFields(runtimeValue.options, runtimeValue.selection, runtimeValue.provider, e.slot, err)...)
		return nil, err
	}
	return client, nil
}

func (e *slotExecution) reloadSlotRuntimeIdentity() (string, string, error) {
	cached := e.slot.getSDKSessionID()
	if e.service.rooms == nil || strings.TrimSpace(e.round.ConversationID) == "" {
		return cached, "", nil
	}
	contextValue, err := e.service.rooms.GetConversationContext(e.ctx, e.round.ConversationID)
	if err != nil {
		return "", "", err
	}
	if contextValue == nil {
		return cached, "", nil
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
		toolSurface, _ := sessionRecord.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
		return resumeID, strings.TrimSpace(toolSurface), nil
	}
	return cached, "", nil
}

func (e *slotExecution) connectRuntimeOnce(
	startup *runtimectx.ClientStartup,
	options agentclient.Options,
	scratchLease *runtimectx.SandboxResourceLease,
) (runtimectx.Client, bool, error) {
	e.logger.Info("准备启动 Room runtime", clientopts.RuntimeStartupLogFields(options)...)
	previousClient := e.service.Runtime.SessionClient(e.slot.RuntimeSessionKey)
	hadWarmSession := e.service.Runtime.HasSession(e.slot.RuntimeSessionKey)
	client, err := startup.GetOrCreateWithLease(e.ctx, options, e.service.factory, scratchLease)
	if err != nil {
		return client, false, err
	}
	transferred, err := startup.BindSandboxLease(scratchLease)
	if err != nil {
		return client, transferred, err
	}
	e.slot.setRuntimeKind(string(e.service.Runtime.RuntimeKind(e.slot.RuntimeSessionKey)))
	e.slot.setClient(client)
	if err = startup.Connect(e.ctx); err != nil {
		return client, transferred, err
	}
	reusedWarmSession := hadWarmSession && previousClient == client
	e.slot.setContextColdStart(roomContextColdStart(options.Session.ResumeID, reusedWarmSession))
	return client, transferred, nil
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
		"requested_runtime_kind", runtimeSelection.RuntimeKind,
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
