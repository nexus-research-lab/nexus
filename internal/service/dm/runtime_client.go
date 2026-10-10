// INPUT: DM session、稳定 execution contract、exact Goal authority、隔离 WorkGraph 保存绑定、Agent runtime 配置与 guidance 队列位置。
// OUTPUT: static/dynamic prompt 分层、跨 backend 工具面 fork、受限临时 Session policy、factory 前 desktop sandbox lease 身份绑定与创建后所有权交接，以及共用同轮 authority 的 Goal/Execution command 与 Subagent control runtime client。
// POS: DM 服务的 runtime client 装配、sandbox lease 失败回收与 owner-private command scope 签发边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// dmClientPreparation 收拢 runtime client 启动后的配置快照，避免扩展返回值参数组。
type dmClientPreparation struct {
	client                 runtimectx.Client
	runtimeKind            string
	runtimeProvider        string
	runtimeModel           string
	toolSurfaceFingerprint string
	forkSourceSessionID    string
	session                protocol.Session
	emotionEnabled         bool
	goalIDForUsage         string
	goalContext            string
	goalObjectiveRevision  *atomic.Int64
	responsibilityState    *runtimectx.ResponsibilityAuthorityState
	sdkSessionIdentity     *runtimectx.SDKSessionIdentityState
	commandReceipts        *nexusmcp.CommandReceiptState
	permissionMode         sdkpermission.Mode
}

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (dmClientPreparation, error) {
	if !request.runtimePreparationOnly {
		s.cancelConnectorRuntimePreparation(agentValue.OwnerUserID, sessionKey)
	}
	forkSourceSessionID := strings.TrimSpace(request.forkSourceSessionID)
	forkMessageID := strings.TrimSpace(request.forkMessageID)
	if (forkSourceSessionID == "") != (forkMessageID == "") {
		return dmClientPreparation{}, errors.New("fork source session id and message id must be provided together")
	}
	startup, err := s.Runtime.BeginClientStartup(ctx, sessionKey, agentValue.OwnerUserID)
	if err != nil {
		return dmClientPreparation{}, err
	}
	defer startup.Close()
	var latestSession *protocol.Session
	if request.runtimePreparationOnly {
		current, currentErr := s.ensureSession(
			ctx,
			agentValue,
			protocol.ParseSessionKey(sessionKey),
			sessionKey,
		)
		if currentErr != nil {
			return dmClientPreparation{}, currentErr
		}
		latestSession = &current
	} else {
		latestSession, _, err = s.Files.ForOwner(agentValue.OwnerUserID).FindSession(
			[]string{agentValue.WorkspacePath},
			sessionKey,
		)
	}
	if err != nil {
		return dmClientPreparation{}, err
	}
	if latestSession != nil {
		sessionItem = *latestSession
	}
	scopedPolicy := protocol.ScopedSessionRuntimePolicy{}
	scopedPolicyActive := false
	if s.scopedSessionPolicy != nil {
		scopedPolicy, scopedPolicyActive, err = s.scopedSessionPolicy.RuntimeEditorPolicy(
			agentValue.OwnerUserID,
			sessionKey,
		)
		if err != nil {
			return dmClientPreparation{}, err
		}
	}
	if !scopedPolicyActive && protocol.SessionPurpose(sessionItem) == protocol.SessionPurposeWorkGraphDistillation {
		scopedPolicy = workGraphDistillationRuntimePolicy()
		scopedPolicyActive = true
	}
	if request.runtimePreparationOnly &&
		sessionItem.ConfigurationVersion != request.expectedConfigurationVersion {
		return dmClientPreparation{}, fmt.Errorf(
			"%w: expected configuration_version=%d actual=%d",
			errConnectorPreparationSuperseded,
			request.expectedConfigurationVersion,
			sessionItem.ConfigurationVersion,
		)
	}
	if request.runtimePreparationOnly && request.expectedConnectorSelection != nil &&
		!protocol.SessionConnectorSelectionFromOptions(sessionItem.Options).Equal(
			*request.expectedConnectorSelection,
		) {
		return dmClientPreparation{}, fmt.Errorf(
			"%w: Connector selection changed during startup",
			errConnectorPreparationSuperseded,
		)
	}
	if forkSourceSessionID == "" && forkMessageID == "" {
		forkSourceSessionID, forkMessageID = pendingConversationFork(sessionItem.Options)
	}
	if (forkSourceSessionID == "") != (forkMessageID == "") {
		return dmClientPreparation{}, errors.New("fork source session id and message id must be provided together")
	}
	forking := forkSourceSessionID != ""
	sessionSettings := protocol.SessionRuntimeSettingsFromOptions(sessionItem.Options)
	permissionMode := resolvePermissionMode(
		request.PermissionMode,
		sessionSettings.PermissionMode,
		agentValue.Options.PermissionMode,
	)
	permissionHandler := request.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
			return s.Permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	permissionHandler = toolpolicy.WithManagedRuntimeAutoApproval(permissionHandler)
	permissionHandler = toolpolicy.WithMalformedInputDeny(permissionHandler)
	var runtimeSkillNames, runtimeDisabledSkillNames []string
	if !scopedPolicyActive || !scopedPolicy.DisableSkills {
		runtimeSkillNames, runtimeDisabledSkillNames, err = s.AgentRuntimeSkills(*agentValue, scopedPolicy.AllowedSkillNames)
		if err != nil {
			return dmClientPreparation{}, err
		}
	}
	dynamicSystemPrompt, err := s.Agents.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		return dmClientPreparation{}, err
	}
	staticSystemPrompt := orchestration.StablePrompt()
	if scopedPolicyActive {
		dynamicSystemPrompt = runtimehost.JoinPromptSections(dynamicSystemPrompt, scopedPolicy.SystemPrompt)
	}
	goalContext, goalIDForUsage, objectiveRevision := "", "", int64(0)
	explicitGoalID := strings.TrimSpace(request.GoalID)
	executionID := request.ExecutionID
	explicitGoalRevision := request.GoalObjectiveRevision
	goalBoundRequest := request.Internal && explicitGoalID != "" && explicitGoalRevision > 0
	if !goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode)) && goalBoundRequest {
		goalContext, goalIDForUsage, objectiveRevision = s.goalRuntimeContext(ctx, sessionKey)
		if goalIDForUsage != explicitGoalID || objectiveRevision != explicitGoalRevision {
			return dmClientPreparation{}, goalsvc.ErrGoalRevisionStale
		}
		goalIDForUsage = explicitGoalID
		objectiveRevision = explicitGoalRevision
	}
	goalAuthority := runtimectx.NewGoalAuthorityState(
		goalIDForUsage,
		objectiveRevision,
		executionID,
	)
	responsibilityState := runtimectx.NewResponsibilityAuthorityState(
		goalAuthority,
		executionID,
		nil,
		nil,
	)
	sdkSessionIdentity := runtimectx.NewSDKSessionIdentityState(
		textutil.PointerValue(sessionItem.SessionID),
	)
	commandReceipts := nexusmcp.NewCommandReceiptState()
	goalObjectiveRevision := goalAuthority.ObjectiveRevisionState()
	sourceContextType := dmMCPSourceContextType(sessionKey, agentValue.AgentID, request)
	if request.goalContinuationAuthority != nil {
		if !trustedDMGoalContinuationAuthority(
			request.goalContinuationAuthority,
			agentValue,
			sessionKey,
			request,
		) {
			return dmClientPreparation{}, errors.New("DM Goal continuation authority does not match the exact owner, Agent, session, Goal, revision, Execution, or round")
		}
		sourceContextType = runtimectx.SourceContextGoalContinuation
	}
	if scopedPolicyActive {
		if request.goalContinuationAuthority != nil {
			return dmClientPreparation{}, errors.New("DM Goal continuation cannot use a scoped WorkGraph policy session")
		}
		sourceContextType = protocol.SessionPurpose(sessionItem)
	}
	commandScopeSessionKey := sessionKey
	workGraphPreviewID := ""
	if sourceContextType == protocol.SessionPurposeWorkGraphDistillation {
		commandScopeSessionKey = strings.TrimSpace(request.WorkGraphSaveSourceSessionKey)
		workGraphPreviewID = strings.TrimSpace(request.InputOptions.Metadata["preview_id"])
		if commandScopeSessionKey == "" || workGraphPreviewID == "" {
			return dmClientPreparation{}, errors.New("WorkGraph distillation round is missing its exact source or preview binding")
		}
	}
	runtimeBuilderContext := runtimectx.WithRuntimeRoundLease(ctx, sessionKey, request.RoundID)
	runtimeCommandContext := runtimectx.RuntimeCommandContext{
		Agent: agentValue, ScopeSessionKey: commandScopeSessionKey, RuntimeSessionKey: sessionKey,
		ExecutionID:        executionID,
		CoordinatorAgentID: agentValue.AgentID,
		RootRoundID:        request.RoundID, AgentRoundID: request.AgentRoundID,
		SourceContextType: sourceContextType, SourceContextID: agentValue.AgentID,
		SourceContextLabel: agentValue.Name, PermissionMode: permissionMode,
		GoalAuthority: goalAuthority, ResponsibilityAuthority: responsibilityState,
		GoalContinuationAuthority: request.goalContinuationAuthority,
		SDKSessionIdentity:        sdkSessionIdentity,
		AutomationRun:             request.AutomationRun.NormalizedCopy(),
		WorkGraphPreviewID:        workGraphPreviewID,
	}
	configurationRuntimeEnv := map[string]string(nil)
	if !request.runtimePreparationOnly && !scopedPolicyActive && s.ConfigurationRuntimeEnv != nil {
		configurationRuntimeEnv, err = s.ConfigurationRuntimeEnv(
			runtimeBuilderContext,
			agentValue,
			sessionKey,
			request.RoundID,
			sourceContextType,
			agentValue.AgentID,
		)
		if err != nil {
			return dmClientPreparation{}, err
		}
	}
	permissionHandler = toolpolicy.WithNexusControlPlaneDeny(permissionHandler, !agentValue.IsMain)
	enabledConnectorIDs := protocol.EffectiveSessionConnectorIDs(
		agentValue.Options.ConnectorIDs,
		sessionItem.Options,
	)
	if scopedPolicyActive && scopedPolicy.DisableConnectors {
		enabledConnectorIDs = []string{}
	}
	dynamicSystemPrompt = runtimehost.JoinPromptSections(
		dynamicSystemPrompt,
		s.connectorRuntimeStatePrompt(ctx, agentValue.OwnerUserID, enabledConnectorIDs),
	)
	mcpContext := runtimectx.WithEnabledConnectorIDs(
		runtimeBuilderContext,
		enabledConnectorIDs,
	)
	mcpContext = runtimectx.WithGoalAuthorityState(mcpContext, goalAuthority)
	mcpContext = runtimectx.WithResponsibilityAuthorityState(
		mcpContext,
		responsibilityState,
	)
	mcpServers := map[string]sdkmcp.ServerConfig(nil)
	if s.MCPServers != nil && !scopedPolicyActive {
		mcpServers = s.MCPServers(
			mcpContext,
			agentValue,
			sessionKey,
			request.RoundID,
			sourceContextType,
			agentValue.AgentID,
			agentValue.Name,
			goalObjectiveRevision,
			permissionMode,
		)
	}
	if (!scopedPolicyActive || sourceContextType == protocol.SessionPurposeWorkGraphEditor ||
		sourceContextType == protocol.SessionPurposeWorkGraphDistillation) &&
		s.NexusMCP != nil {
		mcpServers, err = s.MergeNexusMCPServers(mcpContext, mcpServers, nexusmcp.RoundContext{
			SessionKey: sessionKey, RoundID: request.RoundID, InputContent: request.Content,
			SubagentControl:   s.Runtime.BindSubagentControl(sessionKey, request.RoundID),
			SourceContextType: sourceContextType, SourceContextID: agentValue.AgentID,
			SourceContextLabel: agentValue.Name,
			CommandContext:     runtimeCommandContext, CommandReceipts: commandReceipts,
		})
		if err != nil {
			return dmClientPreparation{}, err
		}
	}
	connectorTurnContext := connectorRuntimeToolPrompt(enabledConnectorIDs, mcpServers)
	dynamicSystemPrompt = runtimehost.JoinPromptSections(dynamicSystemPrompt, connectorTurnContext)
	runtimeSelection, err := s.ResolveAgentRuntimeSelection(ctx, agentValue, sessionItem.Options)
	if err != nil {
		return dmClientPreparation{}, err
	}
	if forking {
		fingerprint := runtimeFingerprintFromSession(sessionItem)
		if fingerprint.kind != "" {
			runtimeSelection.RuntimeKind = fingerprint.kind
		}
		if fingerprint.provider != "" {
			runtimeSelection.Provider = fingerprint.provider
		}
		if fingerprint.model != "" {
			runtimeSelection.Model = fingerprint.model
		}
	}
	if err = s.Agents.EnsureRuntimeVisionSettingsProjection(
		*agentValue,
		runtimeSelection.VisionProvider,
		runtimeSelection.VisionModel,
	); err != nil {
		return dmClientPreparation{}, err
	}
	toolPolicy := request.RuntimeToolPolicy
	if scopedPolicyActive {
		toolPolicy = &scopedPolicy.ToolPolicy
	}
	allowedTools, disallowedTools := resolveDMRuntimeToolPolicy(
		agentValue.Options,
		toolPolicy,
		s.RuntimeImagegenDefaultEnabled(ctx),
	)
	scratchInput := runtimectx.SandboxResourceInput{
		OwnerUserID: agentValue.OwnerUserID,
		SessionKey:  sessionKey,
		RoundID:     request.RoundID,
	}
	scratchLease, err := s.AcquireRuntimeScratch(ctx, runtimeSelection, permissionMode, scratchInput)
	if err != nil {
		return dmClientPreparation{}, err
	}
	// 交给 RuntimeConnect 前的任何返回都由这里释放 scratch。
	defer func() {
		if scratchLease != nil {
			_ = scratchLease.Release()
		}
	}()
	options, _, err := s.BuildAgentRuntimeOptions(ctx, runtimehost.AgentRuntimeOptionsInput{
		Agent:                 agentValue,
		Selection:             runtimeSelection,
		PermissionMode:        permissionMode,
		PermissionHandler:     permissionHandler,
		AllowedTools:          allowedTools,
		DisallowedTools:       disallowedTools,
		SkillIDs:              runtimeSkillNames,
		DisabledSkillIDs:      runtimeDisabledSkillNames,
		AdditionalDirectories: scopedSessionAdditionalDirectories(sessionItem, scopedPolicyActive),
		StablePrompt:          staticSystemPrompt,
		DynamicPrompt:         dynamicSystemPrompt,
		ResumeSessionID:       textutil.FirstNonEmpty(forkSourceSessionID, textutil.PointerValue(sessionItem.SessionID)),
		MCPServers:            mcpServers,
		ConfigurationEnv:      configurationRuntimeEnv,
		Scratch:               scratchLease,
	})
	if err != nil {
		return dmClientPreparation{}, err
	}
	options = s.WithAgentRuntimeHooks(options, sessionKey,
		s.LoggerFor(context.Background()).With("session_key", sessionKey, "agent_id", agentValue.AgentID),
		func(options agentclient.Options) agentclient.Options {
			return s.withInputQueueGuidanceHook(options, sessionKey, workspacestore.InputQueueLocation{
				OwnerUserID:   agentValue.OwnerUserID,
				Scope:         protocol.InputQueueScopeDM,
				WorkspacePath: agentValue.WorkspacePath,
				SessionKey:    sessionKey,
			})
		},
	)
	runtimeProvider := clientopts.ResolvedRuntimeProvider(runtimeSelection.Provider, options)
	toolSurfaceFingerprint, toolSurfaceComplete, err := runtimectx.ModelToolSurfaceFingerprint(ctx, options)
	if err != nil {
		return dmClientPreparation{}, fmt.Errorf("计算 DM runtime 工具面指纹: %w", err)
	}
	storedToolSurface, _ := sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	if strings.TrimSpace(options.Session.ResumeID) != "" &&
		!toolSurfaceComplete &&
		sessionresumesvc.RequiresToolSurfaceFork(
			storedToolSurface,
			toolSurfaceFingerprint,
			len(enabledConnectorIDs) > 0,
		) {
		return dmClientPreparation{}, errors.New("runtime 工具面检查不完整，无法安全 fork 旧 SDK session")
	}
	if request.runtimePreparationOnly && !sessionresumesvc.RequiresToolSurfaceFork(
		storedToolSurface,
		toolSurfaceFingerprint,
		len(enabledConnectorIDs) > 0,
	) {
		return dmClientPreparation{}, nil
	}
	resumeID, toolSurfaceFork := s.resolveReusableSDKSessionID(
		ctx,
		agentValue.WorkspacePath,
		sessionItem,
		runtimeProvider,
		options,
		toolSurfaceFingerprint,
		len(enabledConnectorIDs) > 0,
	)
	if forking && resumeID == "" {
		return dmClientPreparation{}, errors.New("fork source SDK session is unavailable")
	}
	conversationFork := forking
	forking = forking || toolSurfaceFork
	if request.runtimePreparationOnly && !toolSurfaceFork {
		return dmClientPreparation{}, nil
	}
	if request.runtimePreparationOnly {
		latest, latestErr := s.ensureSession(
			ctx,
			agentValue,
			protocol.ParseSessionKey(sessionKey),
			sessionKey,
		)
		if latestErr != nil {
			return dmClientPreparation{}, latestErr
		}
		if latest.ConfigurationVersion != request.expectedConfigurationVersion ||
			(request.expectedConnectorSelection != nil &&
				!protocol.SessionConnectorSelectionFromOptions(latest.Options).Equal(
					*request.expectedConnectorSelection,
				)) {
			return dmClientPreparation{}, fmt.Errorf(
				"%w: Connector selection or configuration version changed before fork",
				errConnectorPreparationSuperseded,
			)
		}
		sessionItem = latest
	}
	options.Session.ResumeID = resumeID
	options.Session.ResumeAt = conversationForkResumeAt(sessionItem.Options, forkMessageID)
	options.Session.Fork = forking
	if toolSurfaceFork {
		retired, retireErr := runtimehost.RetireExistingRuntimeClient(ctx, startup)
		if retireErr != nil && !runtimectx.IsRuntimeTransportClosedError(retireErr) {
			return dmClientPreparation{}, fmt.Errorf("换代 runtime 工具面: %w", retireErr)
		}
		s.LoggerFor(ctx).Info("Session 工具面变化，从旧 transcript fork 新 SDK session",
			"session_key", sessionKey,
			"retired_warm_client", retired,
		)
	}
	s.LoggerFor(ctx).Info("准备启动 DM runtime",
		append(clientopts.RuntimeStartupLogFields(options),
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"requested_runtime_kind", runtimeSelection.RuntimeKind,
			"requested_provider", strings.TrimSpace(runtimeSelection.Provider),
			"requested_model", strings.TrimSpace(runtimeSelection.Model),
			"runtime_provider", runtimeProvider,
		)...,
	)
	clearResume := func() error {
		if _, err := s.clearReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem); err != nil {
			return err
		}
		sdkSessionIdentity.Set("")
		return nil
	}
	if conversationFork {
		// 用户发起的 fork 不能静默退化为全新 SDK session。
		clearResume = nil
	}
	connectScratch := scratchLease
	scratchLease = nil
	client, err := runtimehost.RuntimeConnect{
		Startup:      startup,
		Options:      &options,
		Scratch:      connectScratch,
		ScratchInput: scratchInput,
		Logger:       s.LoggerFor(ctx).With("session_key", sessionKey, "agent_id", agentValue.AgentID),
		Attempt: func(startup *runtimectx.ClientStartup, options agentclient.Options, scratch *runtimectx.SandboxResourceLease) (runtimectx.Client, bool, error) {
			return s.acquireRuntimeClient(ctx, startup, options, scratch)
		},
		ClearResume: clearResume,
	}.Run(ctx)
	if err != nil {
		return dmClientPreparation{}, err
	}
	forking = options.Session.Fork
	forkSourceSessionID = ""
	if forking {
		forkedSessionID := strings.TrimSpace(client.SessionID())
		if forkedSessionID == "" {
			if request.runtimePreparationOnly {
				_, _ = runtimehost.RetireRuntimeClient(startup)
				// Claude Code 等 runtime 直到真实 query 才公布 fork identity；后台预备
				// 不发送隐藏消息，保持下一轮同步 fork 作为正确性边界。
				return dmClientPreparation{}, nil
			}
			// Claude Code 只在首条 query 后通过 init 事件公布 fork identity；
			// 在 round 收到该事件前保持旧 identity/工具面基线不变。
			forkSourceSessionID = resumeID
		} else {
			syncArguments := []sdkSessionSyncConstraint(nil)
			if request.runtimePreparationOnly {
				syncArguments = append(syncArguments, sdkSessionSyncConstraint{
					configurationVersion: request.expectedConfigurationVersion,
					connectorSelection:   request.expectedConnectorSelection,
				})
			}
			updatedSession, syncErr := s.syncSDKSessionIDForOwner(
				ctx,
				agentValue.OwnerUserID,
				agentValue.WorkspacePath,
				sessionItem,
				forkedSessionID,
				strings.TrimSpace(string(options.Runtime.Kind)),
				runtimeProvider,
				strings.TrimSpace(options.Model),
				toolSurfaceFingerprint,
				syncArguments...,
			)
			if syncErr != nil {
				_, _ = runtimehost.RetireRuntimeClient(startup)
				return dmClientPreparation{}, fmt.Errorf("提交 fork SDK session: %w", syncErr)
			}
			sessionItem = updatedSession
			if !forkSessionStateCommitted(sessionItem, forkedSessionID, toolSurfaceFingerprint) {
				forkSourceSessionID = resumeID
			}
		}
	}
	if currentSessionID := strings.TrimSpace(client.SessionID()); currentSessionID != "" {
		sdkSessionIdentity.Set(currentSessionID)
	} else if forkSourceSessionID != "" {
		sdkSessionIdentity.Set("")
	} else {
		sdkSessionIdentity.Set(textutil.PointerValue(sessionItem.SessionID))
	}
	preparation := dmClientPreparation{
		client:                 client,
		runtimeKind:            strings.TrimSpace(string(options.Runtime.Kind)),
		runtimeProvider:        runtimeProvider,
		runtimeModel:           strings.TrimSpace(options.Model),
		toolSurfaceFingerprint: toolSurfaceFingerprint,
		forkSourceSessionID:    forkSourceSessionID,
		session:                sessionItem,
		emotionEnabled:         runtimeSelection.EmotionEnabled,
		goalIDForUsage:         goalIDForUsage,
		goalContext:            goalContext,
		goalObjectiveRevision:  goalObjectiveRevision,
		responsibilityState:    responsibilityState,
		sdkSessionIdentity:     sdkSessionIdentity,
		commandReceipts:        commandReceipts,
		permissionMode:         permissionMode,
	}
	return preparation, nil
}

// trustedDMGoalContinuationAuthority verifies the host-only capability before
// it reaches the shared MCP builder. Request fields are checked as well: a
// continuation plan must not be able to bind a different Goal or round after
// the durable claim has succeeded.
func trustedDMGoalContinuationAuthority(
	authority *runtimectx.GoalContinuationAuthority,
	agentValue *protocol.Agent,
	sessionKey string,
	request Request,
) bool {
	if authority == nil || agentValue == nil || !authority.Valid() || !request.Internal ||
		!isDMGoalContinuationPurpose(request.InputOptions.Purpose) {
		return false
	}
	normalized := authority.Normalized()
	parsed := protocol.ParseSessionKey(sessionKey)
	return normalized.OwnerUserID == strings.TrimSpace(agentValue.OwnerUserID) &&
		normalized.AgentID == strings.TrimSpace(agentValue.AgentID) &&
		normalized.ScopeSessionKey == strings.TrimSpace(sessionKey) &&
		normalized.GoalID == strings.TrimSpace(request.GoalID) &&
		normalized.ObjectiveRevision == request.GoalObjectiveRevision &&
		normalized.ExecutionID == request.ExecutionID &&
		normalized.RootRoundID == strings.TrimSpace(request.RoundID) &&
		parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.ChatType == protocol.RoomTypeDM &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment &&
		strings.TrimSpace(parsed.AgentID) == normalized.AgentID
}

func isDMGoalContinuationPurpose(value string) bool {
	switch strings.TrimSpace(value) {
	case "goal_continuation", "goal_objective_transition_planning":
		return true
	default:
		return false
	}
}

func workGraphDistillationRuntimePolicy() protocol.ScopedSessionRuntimePolicy {
	return protocol.ScopedSessionRuntimePolicy{
		SystemPrompt: "当前是隔离的内部 WorkGraph 保存 Session。只按用户确认的 exact preview_id 调用 distill_workgraph，不读取 workspace、不执行草图任务，也不处理其他请求。",
		ToolPolicy: protocol.RuntimeToolPolicy{
			AllowedTools: []string{"Skill", "mcp__nexus__command"},
			DisallowedTools: []string{
				"Agent", "Edit", "Glob", "Grep", "Task", "WebFetch", "WebSearch",
				"mcp__nexus__show_widget", "mcp__nexus__generate_image", "mcp__nexus__edit_image",
			},
		},
		AllowedSkillNames: []string{"execution-orchestrator"},
		DisableConnectors: true,
	}
}

func conversationForkResumeAt(options map[string]any, forkMessageID string) string {
	if atTail, _ := options[protocol.OptionRuntimeForkAtTranscriptTail].(bool); atTail {
		return ""
	}
	return strings.TrimSpace(forkMessageID)
}

func forkSessionStateCommitted(
	sessionItem protocol.Session,
	sessionID string,
	toolSurfaceFingerprint string,
) bool {
	currentSessionID := textutil.PointerValue(sessionItem.SessionID)
	storedToolSurface, _ := sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	return currentSessionID == strings.TrimSpace(sessionID) &&
		strings.TrimSpace(storedToolSurface) == strings.TrimSpace(toolSurfaceFingerprint)
}

func dmMCPSourceContextType(sessionKey string, agentID string, request Request) string {
	executionOrigin := request.ExecutionOrigin
	if request.trustedQueuedConfigurationContext &&
		executionOrigin == "queue" &&
		trustedDMWebSocketSession(sessionKey, agentID) {
		return "agent"
	}
	if request.TrustedExternalInteractiveContext &&
		executionOrigin == "channel" &&
		trustedExternalDMSession(sessionKey, agentID) {
		return "agent_paired"
	}
	switch {
	case executionOrigin != "":
		return "agent_" + strings.ToLower(executionOrigin)
	case request.Internal:
		return "agent_internal"
	case request.ExternalReplyTarget != nil:
		return "agent_external"
	case !request.TrustedConfigurationContext:
		return "agent_untrusted"
	}
	if !trustedDMWebSocketSession(sessionKey, agentID) {
		return "agent_untrusted"
	}
	return "agent"
}

func trustedExternalDMSession(sessionKey string, agentID string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured ||
		parsed.Kind != protocol.SessionKeyKindAgent ||
		parsed.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(agentID) {
		return false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	return channel != "" &&
		channel != protocol.SessionChannelWebSocket &&
		channel != protocol.SessionChannelInternalSegment
}

func trustedDMWebSocketSession(sessionKey string, agentID string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	return parsed.IsStructured &&
		parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment &&
		parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(agentID)
}

func resolvePermissionMode(
	requestMode sdkpermission.Mode,
	sessionMode string,
	agentMode string,
) sdkpermission.Mode {
	if requestMode != "" {
		return runtimepermission.NormalizeMode(requestMode)
	}
	if sessionMode != "" {
		return runtimepermission.NormalizeMode(sdkpermission.Mode(sessionMode))
	}
	if agentMode != "" {
		return runtimepermission.NormalizeMode(sdkpermission.Mode(agentMode))
	}
	return sdkpermission.ModeDefault
}

func resolveDMRuntimeToolPolicy(
	agentOptions protocol.Options,
	snapshot *protocol.RuntimeToolPolicy,
	imagegenDefaultEnabled bool,
) ([]string, []string) {
	if snapshot != nil {
		return append([]string(nil), snapshot.AllowedTools...), append([]string(nil), snapshot.DisallowedTools...)
	}
	return toolpolicy.WithManagedRuntimeAllowedTools(
		agentOptions.AllowedTools,
		imagegenDefaultEnabled,
	), append([]string(nil), agentOptions.DisallowedTools...)
}

func scopedSessionAdditionalDirectories(session protocol.Session, scoped bool) []string {
	if scoped {
		return nil
	}
	return protocol.SessionAdditionalDirectoriesFromOptions(session.Options)
}

func (s *Service) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, int64) {
	if s.goals == nil {
		return "", "", 0
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if goalsvc.IsAbsent(err) {
			return "", "", 0
		}
		s.LoggerFor(ctx).Warn("读取 Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", "", 0
	}
	goalID := ""
	objectiveRevision := int64(0)
	if goal != nil {
		goalID = strings.TrimSpace(goal.ID)
		objectiveRevision = goal.ObjectiveRevision()
	}
	if strings.TrimSpace(goalContext) == "" {
		return "", goalID, objectiveRevision
	}
	return strings.TrimSpace(goalContext), goalID, objectiveRevision
}

func (s *Service) resolveReusableSDKSessionID(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	provider string,
	options agentclient.Options,
	toolSurfaceFingerprint string,
	forkLegacyToolSurface bool,
) (string, bool) {
	resumeID := strings.TrimSpace(options.Session.ResumeID)
	if resumeID == "" {
		return "", false
	}
	expectedKind := strings.TrimSpace(string(options.Runtime.Kind))
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualKind, hasKindFingerprint := sessionItem.Options[protocol.OptionRuntimeKind].(string)
	actualProvider, hasProviderFingerprint := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	actualModel, hasModelFingerprint := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	actualToolSurface, _ := sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	actualKind = strings.TrimSpace(actualKind)
	actualProvider = strings.TrimSpace(actualProvider)
	actualModel = strings.TrimSpace(actualModel)
	actualToolSurface = strings.TrimSpace(actualToolSurface)
	toolSurfaceFingerprint = strings.TrimSpace(toolSurfaceFingerprint)
	hasFingerprint := hasKindFingerprint || hasProviderFingerprint || hasModelFingerprint
	fingerprintMatches := hasFingerprint &&
		(!hasKindFingerprint || actualKind == expectedKind) &&
		(!hasProviderFingerprint || actualProvider == expectedProvider) &&
		(!hasModelFingerprint || actualModel == expectedModel)
	runtimeChanged := hasKindFingerprint && actualKind != expectedKind
	decision := sessionresumesvc.NewPolicy(
		s.History.ForOwner(authctx.OwnerUserID(ctx)),
	).CanResume(workspacePath, resumeID)
	if decision.Allowed {
		if !runtimeChanged && sessionresumesvc.RequiresToolSurfaceFork(
			actualToolSurface,
			toolSurfaceFingerprint,
			forkLegacyToolSurface,
		) {
			s.LoggerFor(ctx).Info("SDK session 工具面与当前选择不兼容，准备 fork",
				"session_key", sessionItem.SessionKey,
				"sdk_session_id", resumeID,
				"stored_tool_surface_present", actualToolSurface != "",
				"reason", string(decision.Reason),
			)
			return resumeID, true
		}
		if !fingerprintMatches {
			s.LoggerFor(ctx).Info("DM session runtime 配置已变更但 transcript 可恢复，继续 resume",
				"session_key", sessionItem.SessionKey,
				"sdk_session_id", resumeID,
				"old_runtime_kind", actualKind,
				"new_runtime_kind", expectedKind,
				"old_provider", actualProvider,
				"new_provider", expectedProvider,
				"old_model", actualModel,
				"new_model", expectedModel,
				"reason", string(decision.Reason),
			)
		}
		s.persistSDKSessionFingerprint(
			ctx,
			workspacePath,
			sessionItem,
			false,
			expectedKind,
			expectedProvider,
			expectedModel,
			toolSurfaceFingerprint,
		)
		return resumeID, false
	}
	if decision.Err != nil {
		s.LoggerFor(ctx).Warn("检查 SDK session transcript 失败，跳过过期 resume",
			"session_key", sessionItem.SessionKey,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		s.persistSDKSessionFingerprint(
			ctx,
			workspacePath,
			sessionItem,
			true,
			expectedKind,
			expectedProvider,
			expectedModel,
			toolSurfaceFingerprint,
		)
		return "", false
	}

	s.LoggerFor(ctx).Warn("DM SDK session transcript 不存在，跳过过期 resume",
		"session_key", sessionItem.SessionKey,
		"sdk_session_id", decision.SessionID,
		"old_runtime_kind", actualKind,
		"new_runtime_kind", expectedKind,
		"old_provider", actualProvider,
		"new_provider", expectedProvider,
		"old_model", actualModel,
		"new_model", expectedModel,
		"reason", string(decision.Reason),
	)
	s.persistSDKSessionFingerprint(
		ctx,
		workspacePath,
		sessionItem,
		true,
		expectedKind,
		expectedProvider,
		expectedModel,
		toolSurfaceFingerprint,
	)
	return "", false
}

func (s *Service) persistSDKSessionFingerprint(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	clearSessionID bool,
	runtimeKind string,
	provider string,
	model string,
	toolSurfaceFingerprint string,
) {
	if clearSessionID {
		sessionItem.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			sessionItem.TranscriptSessionIDs,
			protocol.SessionTranscriptIDs(sessionItem),
		)
		sessionItem.SessionID = nil
	}
	if sessionItem.Options == nil {
		sessionItem.Options = map[string]any{}
	}
	sessionItem.Options[protocol.OptionRuntimeKind] = strings.TrimSpace(runtimeKind)
	sessionItem.Options[protocol.OptionRuntimeProvider] = strings.TrimSpace(provider)
	sessionItem.Options[protocol.OptionRuntimeModel] = strings.TrimSpace(model)
	sessionItem.Options[protocol.OptionRuntimeToolSurfaceFingerprint] = strings.TrimSpace(toolSurfaceFingerprint)
	var err error
	sessionItem, err = s.preservePersistedSessionTitleForOwner(
		authctx.OwnerUserID(ctx),
		workspacePath,
		sessionItem,
	)
	if err != nil {
		s.LoggerFor(ctx).Error("DM session runtime 配置指纹保留标题失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
		return
	}
	if _, err := s.Files.ForOwner(authctx.OwnerUserID(ctx)).PatchSessionRuntime(
		workspacePath,
		sessionItem,
	); err != nil {
		s.LoggerFor(ctx).Error("DM session runtime 配置指纹更新失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
	}
}

func (s *Service) acquireRuntimeClient(
	ctx context.Context,
	startup *runtimectx.ClientStartup,
	options agentclient.Options,
	scratchLease *runtimectx.SandboxResourceLease,
) (runtimectx.Client, bool, error) {
	client, err := startup.GetOrCreateWithLease(ctx, options, nil, scratchLease)
	if err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "get_or_create", options, err)
		return client, false, err
	}
	transferred, err := startup.BindSandboxLease(scratchLease)
	if err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "bind_sandbox_lease", options, err)
		return client, false, err
	}
	if err := startup.Connect(ctx); err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "connect", options, err)
		return client, transferred, err
	}
	s.LoggerFor(ctx).Info("runtime client connected",
		"session_key", startup.SessionKey(),
		"sdk_session_id", strings.TrimSpace(client.SessionID()),
	)
	return client, transferred, nil
}

func (s *Service) logRuntimeStartupFailure(
	ctx context.Context,
	sessionKey string,
	stage string,
	options agentclient.Options,
	err error,
) {
	s.LoggerFor(ctx).Error("DM runtime 启动失败",
		append(clientopts.RuntimeStartupLogFields(options),
			"session_key", sessionKey,
			"stage", strings.TrimSpace(stage),
			"err", err,
			"error_type", fmt.Sprintf("%T", err),
			"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
		)...,
	)
}
