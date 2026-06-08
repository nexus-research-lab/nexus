package dm

import (
	"context"
	"errors"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
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
) (runtimectx.Client, string, string, string, string, sdkpermission.Mode, error) {
	permissionMode := request.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkpermission.Mode(agentValue.Options.PermissionMode)
	}
	if permissionMode == "" {
		permissionMode = sdkpermission.ModeDefault
	}
	permissionHandler := request.PermissionHandler
	if permissionHandler == nil {
		permissionHandler = func(permissionCtx context.Context, permissionRequest sdkpermission.Request) (sdkpermission.Decision, error) {
			return s.permission.RequestPermission(permissionCtx, sessionKey, permissionRequest)
		}
	}
	permissionHandler = toolpolicy.WithManagedGoalAutoApproval(permissionHandler)
	if err := workspacepkg.EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		return nil, "", "", "", "", permissionMode, err
	}
	appendSystemPrompt, err := s.agents.BuildRuntimePrompt(ctx, agentValue)
	if err != nil {
		return nil, "", "", "", "", permissionMode, err
	}
	goalContext, goalIDForUsage := "", ""
	if !goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(permissionMode)) {
		goalContext, goalIDForUsage = s.goalRuntimeContext(ctx, sessionKey)
	}
	mcpServers := map[string]sdkmcp.ServerConfig(nil)
	if s.mcpServers != nil {
		mcpServers = s.mcpServers(agentValue.AgentID, sessionKey, request.RoundID, "agent", agentValue.AgentID, agentValue.Name)
	}
	runtimeSelection, err := s.resolveAgentRuntimeSelection(ctx, agentValue)
	if err != nil {
		return nil, "", "", "", "", permissionMode, err
	}
	options, err := clientopts.BuildAgentClientOptions(ctx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:              agentValue.WorkspacePath,
		RuntimeKind:                runtimeSelection.RuntimeKind,
		Provider:                   runtimeSelection.Provider,
		Model:                      runtimeSelection.Model,
		PermissionMode:             permissionMode,
		PermissionHandler:          permissionHandler,
		AllowedTools:               toolpolicy.WithManagedGoalAllowedTools(agentValue.Options.AllowedTools),
		DisallowedTools:            agentValue.Options.DisallowedTools,
		SettingSources:             agentValue.Options.SettingSources,
		AppendSystemPrompt:         appendSystemPrompt,
		ResumeSessionID:            dmdomain.StringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:          agentValue.Options.MaxThinkingTokens,
		MaxTurns:                   agentValue.Options.MaxTurns,
		MCPServers:                 mcpServers,
		AgentSDKDiagnosticsEnabled: runtimeSelection.AgentSDKDiagnosticsEnabled,
	})
	if err != nil {
		return nil, "", "", "", "", permissionMode, err
	}
	options = s.runtime.WithGuidanceHook(options, sessionKey)
	options = s.withInputQueueGuidanceHook(options, sessionKey, workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}, sessionItem)
	options = s.withRuntimeDiagnosticsLogger(options, sessionKey, agentValue.AgentID)
	runtimeProvider := resolvedRuntimeProvider(runtimeSelection.Provider, options)
	options.Session.ResumeID = s.resolveReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, runtimeProvider, options)
	client, err := s.acquireRuntimeClient(ctx, sessionKey, options)
	if err != nil {
		if !shouldRetryDMClientWithoutResume(options.Session.ResumeID, err) {
			return nil, "", "", "", "", permissionMode, err
		}
		s.loggerFor(ctx).Warn("DM SDK session resume 失效，清除后重试",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"sdk_session_id", options.Session.ResumeID,
			"err", err,
		)
		if closeErr := s.runtime.CloseSession(ctx, sessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			return nil, "", "", "", "", permissionMode, closeErr
		}
		if _, clearErr := s.clearReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem); clearErr != nil {
			return nil, "", "", "", "", permissionMode, clearErr
		}
		options.Session.ResumeID = ""
		client, err = s.acquireRuntimeClient(ctx, sessionKey, options)
		if err != nil {
			return nil, "", "", "", "", permissionMode, err
		}
	}
	return client, runtimeProvider, strings.TrimSpace(options.Model), goalIDForUsage, goalContext, permissionMode, nil
}

func (s *Service) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string) {
	if s.goals == nil {
		return "", ""
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return "", ""
		}
		s.loggerFor(ctx).Warn("读取 Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", ""
	}
	goalID := goalIDForRuntimeUsage(goal)
	if strings.TrimSpace(goalContext) == "" {
		return "", goalID
	}
	return strings.TrimSpace(goalContext), goalID
}

func goalIDForRuntimeUsage(goal *protocol.Goal) string {
	if goal == nil {
		return ""
	}
	return strings.TrimSpace(goal.ID)
}

func (s *Service) resolveAgentRuntimeSelection(
	ctx context.Context,
	agentValue *protocol.Agent,
) (runtimeselectionsvc.Selection, error) {
	return runtimeselectionsvc.NewService(s.prefs).Resolve(ctx, runtimeselectionsvc.Request{
		Agent: agentValue,
	})
}

func resolvedRuntimeProvider(provider string, options agentclient.Options) string {
	if options.Env != nil {
		if resolved := strings.TrimSpace(options.Env[clientopts.NexusRuntimeProviderEnvName]); resolved != "" {
			return resolved
		}
	}
	return strings.TrimSpace(provider)
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
	expectedProvider := strings.TrimSpace(provider)
	expectedModel := strings.TrimSpace(options.Model)
	actualProvider, hasProviderFingerprint := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	actualModel, hasModelFingerprint := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	actualProvider = strings.TrimSpace(actualProvider)
	actualModel = strings.TrimSpace(actualModel)
	hasFingerprint := hasProviderFingerprint || hasModelFingerprint
	if hasFingerprint &&
		(!hasProviderFingerprint || actualProvider == expectedProvider) &&
		(!hasModelFingerprint || actualModel == expectedModel) {
		if !hasProviderFingerprint || !hasModelFingerprint {
			s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, false, expectedProvider, expectedModel)
		}
		return resumeID
	}
	if !hasFingerprint {
		s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, false, expectedProvider, expectedModel)
		return resumeID
	}
	s.loggerFor(ctx).Warn("DM session runtime 配置已变更，跳过过期 SDK session resume",
		"session_key", sessionItem.SessionKey,
		"old_provider", actualProvider,
		"new_provider", expectedProvider,
		"old_model", actualModel,
		"new_model", expectedModel,
	)
	s.persistSDKSessionFingerprint(ctx, workspacePath, sessionItem, true, expectedProvider, expectedModel)
	return ""
}

func (s *Service) persistSDKSessionFingerprint(
	ctx context.Context,
	workspacePath string,
	sessionItem protocol.Session,
	clearSessionID bool,
	provider string,
	model string,
) {
	if clearSessionID {
		sessionItem.SessionID = nil
	}
	if sessionItem.Options == nil {
		sessionItem.Options = map[string]any{}
	}
	sessionItem.Options[protocol.OptionRuntimeProvider] = strings.TrimSpace(provider)
	sessionItem.Options[protocol.OptionRuntimeModel] = strings.TrimSpace(model)
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
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	return client, nil
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
		logger.Warn("Agent SDK stderr", "stderr", normalizedLine)
	}
	previousDiagnostics := options.Callbacks.Diagnostics
	if !runtimectx.AgentSDKDiagnosticsEnabled(options.Env) {
		if previousDiagnostics == nil {
			options.Callbacks.Diagnostics = func(agentclient.DiagnosticEvent) {}
		}
		return options
	}
	options.Callbacks.Diagnostics = func(event agentclient.DiagnosticEvent) {
		if previousDiagnostics != nil {
			previousDiagnostics(event)
		}
		logger.Info("Agent SDK diagnostics",
			"component", strings.TrimSpace(event.Component),
			"event", strings.TrimSpace(event.Event),
			"attrs", event.Attributes,
		)
	}
	logger.Info("Agent SDK diagnostics 已启用",
		"diagnostics_env", runtimectx.AgentSDKDiagnosticsValue(options.Env),
		"provider_debug_body", runtimectx.AgentSDKProviderDebugBodyValue(options.Env),
	)
	return options
}

func shouldRetryDMClientWithoutResume(resumeID string, err error) bool {
	return strings.TrimSpace(resumeID) != "" && runtimectx.IsRuntimeTransportClosedError(err)
}
