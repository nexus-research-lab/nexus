// INPUT: DM session、Agent runtime 配置与 guidance 队列位置。
// OUTPUT: 可复用并带诊断、权限和 PostToolUse hooks 的 runtime client。
// POS: DM 服务的 runtime client 装配边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
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

func (s *Service) ensureClient(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (runtimectx.Client, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode, error) {
	permissionMode := resolvePermissionMode(request.PermissionMode, agentValue.Options.PermissionMode)
	permissionHandler := request.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
			return s.permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	permissionHandler = toolpolicy.WithManagedGoalAutoApproval(permissionHandler)
	permissionHandler = toolpolicy.WithMalformedInputDeny(permissionHandler)
	if err := workspacepkg.EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		return nil, "", "", "", "", "", nil, permissionMode, err
	}
	appendSystemPrompt, err := s.agents.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		return nil, "", "", "", "", "", nil, permissionMode, err
	}
	goalContext, goalIDForUsage, objectiveRevision := "", "", int64(0)
	if !goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode)) {
		goalContext, goalIDForUsage, objectiveRevision = s.goalRuntimeContext(ctx, sessionKey)
	}
	if request.Internal && request.GoalObjectiveRevision > 0 {
		boundGoalID := strings.TrimSpace(request.GoalID)
		boundObjectiveRevision := request.GoalObjectiveRevision
		if strings.TrimSpace(goalIDForUsage) != boundGoalID || objectiveRevision != boundObjectiveRevision {
			return nil, "", "", "", "", "", nil, permissionMode, goalsvc.ErrGoalRevisionStale
		}
		if boundGoalID != "" {
			goalIDForUsage = boundGoalID
		}
		objectiveRevision = boundObjectiveRevision
	}
	goalObjectiveRevision := &atomic.Int64{}
	goalObjectiveRevision.Store(objectiveRevision)
	mcpServers := map[string]sdkmcp.ServerConfig(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(agentValue.AgentID, sessionKey, request.RoundID, "agent", agentValue.AgentID, agentValue.Name, goalObjectiveRevision)
	}
	runtimeSelection, err := s.resolveAgentRuntimeSelection(ctx, agentValue)
	if err != nil {
		return nil, "", "", "", "", "", nil, permissionMode, err
	}
	if err = agentsvc.EnsureRuntimeVisionSettingsProjection(
		agentValue.WorkspacePath,
		runtimeSelection.VisionProvider,
		runtimeSelection.VisionModel,
	); err != nil {
		return nil, "", "", "", "", "", nil, permissionMode, err
	}
	options, err := clientopts.BuildAgentClientOptions(ctx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              agentValue.WorkspacePath,
		RuntimeKind:                runtimeSelection.RuntimeKind,
		Provider:                   runtimeSelection.Provider,
		Model:                      runtimeSelection.Model,
		VisionProvider:             runtimeSelection.VisionProvider,
		VisionModel:                runtimeSelection.VisionModel,
		PermissionMode:             permissionMode,
		PermissionHandler:          permissionHandler,
		AllowedTools:               toolpolicy.WithManagedRuntimeAllowedTools(agentValue.Options.AllowedTools, s.runtimeImagegenDefaultEnabled(ctx)),
		DisallowedTools:            agentValue.Options.DisallowedTools,
		SettingSources:             agentValue.Options.SettingSources,
		AppendSystemPrompt:         appendSystemPrompt,
		ResumeSessionID:            dmdomain.StringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:          agentValue.Options.MaxThinkingTokens,
		MaxTurns:                   agentValue.Options.MaxTurns,
		MCPServers:                 mcpServers,
		AgentSDKDiagnosticsEnabled: runtimeSelection.AgentSDKDiagnosticsEnabled,
		ToolSearchEnabled:          runtimeSelection.ToolSearchEnabled,
		WebSearch:                  runtimeSelection.WebSearch,
	})
	if err != nil {
		return nil, "", "", "", "", "", nil, permissionMode, err
	}
	options = s.runtime.WithGuidanceHook(options, sessionKey)
	options = s.withInputQueueGuidanceHook(options, sessionKey, workspacestore.InputQueueLocation{
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
	client, err := s.acquireRuntimeClient(ctx, sessionKey, options)
	if err != nil {
		if strings.TrimSpace(options.Session.ResumeID) == "" || !runtimectx.IsRuntimeTransportClosedError(err) {
			return nil, "", "", "", "", "", nil, permissionMode, err
		}
		s.loggerFor(ctx).Warn("DM SDK session resume 失效，清除后重试",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"sdk_session_id", options.Session.ResumeID,
			"err", err,
		)
		if closeErr := s.runtime.CloseSession(ctx, sessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			return nil, "", "", "", "", "", nil, permissionMode, closeErr
		}
		if _, clearErr := s.clearReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem); clearErr != nil {
			return nil, "", "", "", "", "", nil, permissionMode, clearErr
		}
		options.Session.ResumeID = ""
		client, err = s.acquireRuntimeClient(ctx, sessionKey, options)
		if err != nil {
			return nil, "", "", "", "", "", nil, permissionMode, err
		}
	}
	return client, strings.TrimSpace(string(options.Runtime.Kind)), runtimeProvider, strings.TrimSpace(options.Model), goalIDForUsage, goalContext, goalObjectiveRevision, permissionMode, nil
}

func resolvePermissionMode(requestMode sdkpermission.Mode, agentMode string) sdkpermission.Mode {
	if requestMode != "" {
		return runtimepermission.NormalizeMode(requestMode)
	}
	if agentMode != "" {
		return runtimepermission.NormalizeMode(sdkpermission.Mode(agentMode))
	}
	return sdkpermission.ModeDefault
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
) (runtimeselectionsvc.Selection, error) {
	return runtimeselectionsvc.NewService(s.prefs).Resolve(ctx, runtimeselectionsvc.Request{
		Agent: agentValue,
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
	decision := sessionresumesvc.NewPolicy(s.history).CanResume(workspacePath, resumeID)
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
		sessionItem.SessionID = nil
	}
	if sessionItem.Options == nil {
		sessionItem.Options = map[string]any{}
	}
	sessionItem.Options[protocol.OptionRuntimeKind] = strings.TrimSpace(runtimeKind)
	sessionItem.Options[protocol.OptionRuntimeProvider] = strings.TrimSpace(provider)
	sessionItem.Options[protocol.OptionRuntimeModel] = strings.TrimSpace(model)
	var err error
	sessionItem, err = s.preservePersistedSessionTitle(workspacePath, sessionItem)
	if err != nil {
		s.loggerFor(ctx).Error("DM session runtime 配置指纹保留标题失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
		return
	}
	if _, err := s.files.UpsertSession(workspacePath, sessionItem); err != nil {
		s.loggerFor(ctx).Error("DM session runtime 配置指纹更新失败",
			"session_key", sessionItem.SessionKey,
			"err", err,
		)
	}
}

func (s *Service) acquireRuntimeClient(
	ctx context.Context,
	sessionKey string,
	options agentclient.Options,
) (runtimectx.Client, error) {
	client, err := s.runtime.GetOrCreate(ctx, sessionKey, options)
	if err != nil {
		s.logRuntimeStartupFailure(ctx, sessionKey, "get_or_create", options, err)
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		s.logRuntimeStartupFailure(ctx, sessionKey, "connect", options, err)
		return nil, err
	}
	s.loggerFor(ctx).Info("runtime client connected",
		"session_key", sessionKey,
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
