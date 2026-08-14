// INPUT: DM session、稳定 execution contract、exact Goal authority、Agent runtime 配置与 guidance 队列位置。
// OUTPUT: static/dynamic prompt 分层，并让 Goal/Execution MCP 共用同一 round authority 的换代安全 runtime client。
// POS: DM 服务的 runtime client 装配边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// dmClientPreparation 收拢 runtime client 启动后的配置快照，避免扩展返回值参数组。
type dmClientPreparation struct {
	client                runtimectx.Client
	runtimeKind           string
	runtimeProvider       string
	runtimeModel          string
	emotionEnabled        bool
	goalIDForUsage        string
	goalContext           string
	goalObjectiveRevision *atomic.Int64
	permissionMode        sdkpermission.Mode
}

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (dmClientPreparation, error) {
	startup, err := s.runtime.BeginClientStartup(ctx, sessionKey, agentValue.OwnerUserID)
	if err != nil {
		return dmClientPreparation{}, err
	}
	defer startup.Close()
	latestSession, _, err := s.files.ForOwner(agentValue.OwnerUserID).FindSession(
		[]string{agentValue.WorkspacePath},
		sessionKey,
	)
	if err != nil {
		return dmClientPreparation{}, err
	}
	if latestSession != nil {
		sessionItem = *latestSession
	}
	sessionSettings := protocol.SessionRuntimeSettingsFromOptions(sessionItem.Options)
	permissionMode := resolvePermissionMode(
		request.PermissionMode,
		sessionSettings.PermissionMode,
		agentValue.Options.PermissionMode,
	)
	permissionHandler := request.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
			return s.permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	permissionHandler = toolpolicy.WithManagedRuntimeAutoApproval(permissionHandler)
	permissionHandler = toolpolicy.WithMalformedInputDeny(permissionHandler)
	if err := workspacepkg.EnsureUserSkillLibrary(s.config, agentValue.OwnerUserID); err != nil {
		return dmClientPreparation{}, err
	}
	if err := workspacepkg.EnsureInitializedForAgent(s.config, *agentValue); err != nil {
		return dmClientPreparation{}, err
	}
	runtimeSkillNames, err := workspacepkg.RuntimeSkillNamesForAgent(s.config, *agentValue)
	if err != nil {
		return dmClientPreparation{}, err
	}
	runtimeDisabledSkillNames, err := workspacepkg.RuntimeDisabledSkillNamesForAgent(
		s.config,
		*agentValue,
	)
	if err != nil {
		return dmClientPreparation{}, err
	}
	configurationRoleSkill := workspacepkg.ConfigurationSkillAgentSelf
	if agentValue.IsMain {
		configurationRoleSkill = workspacepkg.ConfigurationSkillOwnerMain
	}
	runtimeSkillNames, runtimeDisabledSkillNames = workspacepkg.WithRuntimeConfigurationRoleSkill(
		runtimeSkillNames,
		runtimeDisabledSkillNames,
		configurationRoleSkill,
	)
	dynamicSystemPrompt, err := s.agents.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		return dmClientPreparation{}, err
	}
	staticSystemPrompt := orchestration.StablePrompt()
	goalContext, goalIDForUsage, objectiveRevision := "", "", int64(0)
	explicitGoalID := strings.TrimSpace(request.GoalID)
	explicitGoalRevision := request.GoalObjectiveRevision
	goalBoundRequest := request.Internal && explicitGoalID != "" && explicitGoalRevision > 0
	if !goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode)) && goalBoundRequest {
		goalContext, goalIDForUsage, objectiveRevision = s.goalRuntimeContext(ctx, sessionKey)
		if strings.TrimSpace(goalIDForUsage) != explicitGoalID || objectiveRevision != explicitGoalRevision {
			return dmClientPreparation{}, goalsvc.ErrGoalRevisionStale
		}
		goalIDForUsage = explicitGoalID
		objectiveRevision = explicitGoalRevision
	}
	goalAuthority := runtimectx.NewGoalAuthorityState(
		goalIDForUsage,
		objectiveRevision,
		strings.TrimSpace(request.ExecutionID),
	)
	goalObjectiveRevision := goalAuthority.ObjectiveRevisionState()
	sourceContextType := dmMCPSourceContextType(sessionKey, agentValue.AgentID, request)
	permissionHandler = toolpolicy.WithNexusControlPlaneDeny(permissionHandler, !agentValue.IsMain)
	mcpServers := map[string]sdkmcp.ServerConfig(nil)
	if s.mcpServers != nil {
		mcpContext := runtimectx.WithMCPRoundLease(ctx, sessionKey, request.RoundID)
		mcpContext = runtimectx.WithEnabledConnectorIDs(
			mcpContext,
			protocol.EffectiveSessionConnectorIDs(
				agentValue.Options.ConnectorIDs,
				sessionItem.Options,
			),
		)
		mcpContext = runtimectx.WithGoalAuthorityState(mcpContext, goalAuthority)
		mcpServers = s.mcpServers(
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
	if s.executionMCPServers != nil {
		overlay := s.executionMCPServers(ctx, runtimectx.ExecutionToolContext{
			Agent:              agentValue,
			ScopeSessionKey:    sessionKey,
			RuntimeSessionKey:  sessionKey,
			ExecutionID:        strings.TrimSpace(request.ExecutionID),
			CoordinatorAgentID: agentValue.AgentID,
			RootRoundID:        request.RoundID,
			AgentRoundID:       request.AgentRoundID,
			SourceContextType:  "agent",
			SourceContextID:    agentValue.AgentID,
			PermissionMode:     permissionMode,
			GoalAuthority:      goalAuthority,
			AutomationRun:      cloneAutomationRunContext(request.AutomationRun),
		})
		if len(overlay) > 0 && mcpServers == nil {
			mcpServers = make(map[string]sdkmcp.ServerConfig, len(overlay))
		}
		for name, server := range overlay {
			mcpServers[name] = server
		}
	}
	runtimeSelection, err := s.resolveAgentRuntimeSelection(
		ctx,
		agentValue,
		sessionItem.Options,
	)
	if err != nil {
		return dmClientPreparation{}, err
	}
	if err = s.agents.EnsureRuntimeVisionSettingsProjection(
		*agentValue,
		runtimeSelection.VisionProvider,
		runtimeSelection.VisionModel,
	); err != nil {
		return dmClientPreparation{}, err
	}
	allowedTools, disallowedTools := resolveDMRuntimeToolPolicy(
		agentValue.Options,
		request.RuntimeToolPolicy,
		s.runtimeImagegenDefaultEnabled(ctx),
	)
	options, err := clientopts.BuildAgentClientOptions(ctx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              agentValue.WorkspacePath,
		OwnerUserID:                agentValue.OwnerUserID,
		IsMainAgent:                agentValue.IsMain,
		RuntimeKind:                runtimeSelection.RuntimeKind,
		Provider:                   runtimeSelection.Provider,
		Model:                      runtimeSelection.Model,
		VisionProvider:             runtimeSelection.VisionProvider,
		VisionModel:                runtimeSelection.VisionModel,
		PermissionMode:             permissionMode,
		PermissionHandler:          permissionHandler,
		AllowedTools:               allowedTools,
		DisallowedTools:            disallowedTools,
		SkillIDs:                   runtimeSkillNames,
		DisabledSkillIDs:           runtimeDisabledSkillNames,
		SkillDirectories:           workspacepkg.SkillLibraryRoots(s.config, agentValue.OwnerUserID),
		AdditionalDirectories:      protocol.SessionAdditionalDirectoriesFromOptions(sessionItem.Options),
		SettingSources:             agentValue.Options.SettingSources,
		AppendSystemPrompt:         joinDMRuntimePrompts(staticSystemPrompt, dynamicSystemPrompt),
		AppendSystemPromptStatic:   staticSystemPrompt,
		AppendSystemPromptDynamic:  dynamicSystemPrompt,
		ResumeSessionID:            dmdomain.StringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:          agentValue.Options.MaxThinkingTokens,
		MaxTurns:                   agentValue.Options.MaxTurns,
		MCPServers:                 mcpServers,
		AgentMCPServers:            agentValue.Options.MCPServers,
		AgentSDKDiagnosticsEnabled: runtimeSelection.AgentSDKDiagnosticsEnabled,
		ToolSearchEnabled:          runtimeSelection.ToolSearchEnabled,
		WebSearch:                  runtimeSelection.WebSearch,
		RuntimeIsolationMode:       s.config.RuntimeIsolationMode,
		RuntimeLauncherPath:        s.config.RuntimeLauncherPath,
	})
	if err != nil {
		return dmClientPreparation{}, err
	}
	options = s.runtime.WithGuidanceHook(options, sessionKey)
	options = s.runtime.WithSubagentAdmissionHooks(options, sessionKey)
	options = s.withInputQueueGuidanceHook(options, sessionKey, workspacestore.InputQueueLocation{
		OwnerUserID:   agentValue.OwnerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	})
	options = s.withRuntimeDiagnosticsLogger(options, sessionKey, agentValue.AgentID)
	runtimeProvider := clientopts.ResolvedRuntimeProvider(runtimeSelection.Provider, options)
	options.Session.ResumeID = s.resolveReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, runtimeProvider, options)
	s.loggerFor(ctx).Info("准备启动 DM runtime",
		append(clientopts.RuntimeStartupLogFields(options),
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"requested_runtime_kind", strings.TrimSpace(runtimeSelection.RuntimeKind),
			"requested_provider", strings.TrimSpace(runtimeSelection.Provider),
			"requested_model", strings.TrimSpace(runtimeSelection.Model),
			"runtime_provider", runtimeProvider,
		)...,
	)
	client, err := s.acquireRuntimeClient(ctx, startup, options)
	if err != nil {
		retired, closeErr := retireDMRuntimeClient(ctx, startup)
		if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			s.loggerFor(ctx).Warn("清理启动失败的 DM runtime 返回错误",
				"session_key", sessionKey,
				"agent_id", agentValue.AgentID,
				"startup_err", err,
				"cleanup_err", closeErr,
			)
		}
		if strings.TrimSpace(options.Session.ResumeID) == "" || !runtimectx.IsRuntimeTransportClosedError(err) {
			return dmClientPreparation{}, err
		}
		s.loggerFor(ctx).Warn("DM SDK session resume 失效，清除后重试",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"sdk_session_id", options.Session.ResumeID,
			"err", err,
		)
		if !retired {
			return dmClientPreparation{}, err
		}
		if _, clearErr := s.clearReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem); clearErr != nil {
			return dmClientPreparation{}, clearErr
		}
		options.Session.ResumeID = ""
		if errors.Is(closeErr, context.Canceled) || errors.Is(closeErr, context.DeadlineExceeded) {
			return dmClientPreparation{}, err
		}
		client, err = s.acquireRuntimeClient(ctx, startup, options)
		if err != nil {
			if _, cleanupErr := retireDMRuntimeClient(ctx, startup); cleanupErr != nil &&
				!runtimectx.IsRuntimeTransportClosedError(cleanupErr) {
				s.loggerFor(ctx).Warn("清理重试失败的 DM runtime 返回错误",
					"session_key", sessionKey,
					"agent_id", agentValue.AgentID,
					"startup_err", err,
					"cleanup_err", cleanupErr,
				)
			}
			return dmClientPreparation{}, err
		}
	}
	return dmClientPreparation{
		client:                client,
		runtimeKind:           strings.TrimSpace(string(options.Runtime.Kind)),
		runtimeProvider:       runtimeProvider,
		runtimeModel:          strings.TrimSpace(options.Model),
		emotionEnabled:        runtimeSelection.EmotionEnabled,
		goalIDForUsage:        goalIDForUsage,
		goalContext:           goalContext,
		goalObjectiveRevision: goalObjectiveRevision,
		permissionMode:        permissionMode,
	}, nil
}

func joinDMRuntimePrompts(stable string, dynamic string) string {
	parts := make([]string, 0, 2)
	for _, prompt := range []string{stable, dynamic} {
		if prompt = strings.TrimSpace(prompt); prompt != "" {
			parts = append(parts, prompt)
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func retireDMRuntimeClient(ctx context.Context, startup *runtimectx.ClientStartup) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	return startup.RetireCurrent(closeCtx)
}

func dmMCPSourceContextType(sessionKey string, agentID string, request Request) string {
	if request.trustedQueuedConfigurationContext &&
		strings.TrimSpace(request.ExecutionOrigin) == "queue" &&
		trustedDMWebSocketSession(sessionKey, agentID) {
		return "agent"
	}
	if request.TrustedExternalInteractiveContext &&
		strings.TrimSpace(request.ExecutionOrigin) == "channel" &&
		trustedExternalDMSession(sessionKey, agentID) {
		return "agent_paired"
	}
	switch {
	case strings.TrimSpace(request.ExecutionOrigin) != "":
		return "agent_" + strings.ToLower(strings.TrimSpace(request.ExecutionOrigin))
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

func (s *Service) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, int64) {
	if s.goals == nil {
		return "", "", 0
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return "", "", 0
		}
		s.loggerFor(ctx).Warn("读取 Goal runtime context 失败", "session_key", sessionKey, "err", err)
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

func (s *Service) resolveAgentRuntimeSelection(
	ctx context.Context,
	agentValue *protocol.Agent,
	sessionOptions map[string]any,
) (runtimeselectionsvc.Selection, error) {
	return runtimeselectionsvc.NewServiceWithRuntimeConfigResolver(s.prefs, s.providers).Resolve(ctx, runtimeselectionsvc.Request{
		Agent:          agentValue,
		SessionOptions: sessionOptions,
	})
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

func (s *Service) resolveReusableSDKSessionID(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	provider string,
	options agentclient.Options,
) string {
	resumeID := strings.TrimSpace(options.Session.ResumeID)
	if resumeID == "" {
		return ""
	}
	expectedKind := strings.TrimSpace(string(options.Runtime.Kind))
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualKind, hasKindFingerprint := sessionItem.Options[protocol.OptionRuntimeKind].(string)
	actualProvider, hasProviderFingerprint := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	actualModel, hasModelFingerprint := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	actualKind = strings.TrimSpace(actualKind)
	actualProvider = strings.TrimSpace(actualProvider)
	actualModel = strings.TrimSpace(actualModel)
	hasFingerprint := hasKindFingerprint || hasProviderFingerprint || hasModelFingerprint
	fingerprintMatches := hasFingerprint &&
		(!hasKindFingerprint || actualKind == expectedKind) &&
		(!hasProviderFingerprint || actualProvider == expectedProvider) &&
		(!hasModelFingerprint || actualModel == expectedModel)
	decision := sessionresumesvc.NewPolicy(
		s.history.ForOwner(authctx.OwnerUserID(ctx)),
	).CanResume(workspacePath, resumeID)
	if decision.Allowed {
		if !fingerprintMatches {
			s.loggerFor(ctx).Info("DM session runtime 配置已变更但 transcript 可恢复，继续 resume",
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
		s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, false, expectedKind, expectedProvider, expectedModel)
		return resumeID
	}
	if decision.Err != nil {
		s.loggerFor(ctx).Warn("检查 SDK session transcript 失败，跳过过期 resume",
			"session_key", sessionItem.SessionKey,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, true, expectedKind, expectedProvider, expectedModel)
		return ""
	}

	s.loggerFor(ctx).Warn("DM SDK session transcript 不存在，跳过过期 resume",
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
	s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, true, expectedKind, expectedProvider, expectedModel)
	return ""
}

func (s *Service) persistSDKSessionFingerprint(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	clearSessionID bool,
	runtimeKind string,
	provider string,
	model string,
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
	var err error
	sessionItem, err = s.preservePersistedSessionTitleForOwner(
		authctx.OwnerUserID(ctx),
		workspacePath,
		sessionItem,
	)
	if err != nil {
		s.loggerFor(ctx).Error("DM session runtime 配置指纹保留标题失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
		return
	}
	if _, err := s.files.ForOwner(authctx.OwnerUserID(ctx)).PatchSessionRuntime(
		workspacePath,
		sessionItem,
	); err != nil {
		s.loggerFor(ctx).Error("DM session runtime 配置指纹更新失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
	}
}

func (s *Service) acquireRuntimeClient(
	ctx context.Context,
	startup *runtimectx.ClientStartup,
	options agentclient.Options,
) (runtimectx.Client, error) {
	client, err := startup.GetOrCreateWithFactory(ctx, options, nil)
	if err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "get_or_create", options, err)
		return client, err
	}
	if err := startup.Connect(ctx); err != nil {
		s.logRuntimeStartupFailure(ctx, startup.SessionKey(), "connect", options, err)
		return client, err
	}
	s.loggerFor(ctx).Info("runtime client connected",
		"session_key", startup.SessionKey(),
		"sdk_session_id", strings.TrimSpace(client.SessionID()),
	)
	return client, nil
}

func (s *Service) logRuntimeStartupFailure(
	ctx context.Context,
	sessionKey string,
	stage string,
	options agentclient.Options,
	err error,
) {
	s.loggerFor(ctx).Error("DM runtime 启动失败",
		append(clientopts.RuntimeStartupLogFields(options),
			"session_key", sessionKey,
			"stage", strings.TrimSpace(stage),
			"err", err,
			"error_type", fmt.Sprintf("%T", err),
			"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
		)...,
	)
}

func (s *Service) withRuntimeDiagnosticsLogger(
	options agentclient.Options,
	sessionKey string,
	agentID string,
) agentclient.Options {
	logger := s.loggerFor(context.Background()).With(
		"session_key", sessionKey,
		"agent_id", agentID,
	)
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
