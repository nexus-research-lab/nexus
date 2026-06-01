package dm

import (
	"context"
	"errors"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
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
	runtimeProvider, runtimeModel, err := s.resolveAgentRuntimeSelection(ctx, agentValue)
	if err != nil {
		return nil, "", "", "", "", permissionMode, err
	}
	options, err := clientopts.BuildAgentClientOptions(ctx, s.providers, clientopts.AgentClientOptionsInput{
		WorkspacePath:      agentValue.WorkspacePath,
		Provider:           runtimeProvider,
		Model:              runtimeModel,
		PermissionMode:     permissionMode,
		PermissionHandler:  permissionHandler,
		AllowedTools:       toolpolicy.WithManagedGoalAllowedTools(agentValue.Options.AllowedTools),
		DisallowedTools:    agentValue.Options.DisallowedTools,
		SettingSources:     agentValue.Options.SettingSources,
		AppendSystemPrompt: appendSystemPrompt,
		ResumeSessionID:    dmdomain.StringPointerValue(sessionItem.SessionID),
		MaxThinkingTokens:  agentValue.Options.MaxThinkingTokens,
		MaxTurns:           agentValue.Options.MaxTurns,
		MCPServers:         mcpServers,
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
	runtimeProvider = resolvedRuntimeProvider(runtimeProvider, options)
	options.Session.ResumeID = s.resolveReusableSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, runtimeProvider, options)
	client, err := s.acquireRuntimeClient(ctx, sessionKey, options)
	if err != nil {
		return nil, "", "", "", "", permissionMode, err
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
) (string, string, error) {
	if agentValue == nil {
		return "", "", nil
	}
	provider := strings.TrimSpace(agentValue.Options.Provider)
	model := strings.TrimSpace(agentValue.Options.Model)
	if provider != "" && model != "" {
		return provider, model, nil
	}
	defaultProvider, defaultModel, err := s.preferenceRuntimeSelection(ctx, agentValue)
	if err != nil || defaultProvider != "" || defaultModel != "" {
		return defaultProvider, defaultModel, err
	}
	return provider, model, nil
}

func (s *Service) preferenceRuntimeSelection(
	ctx context.Context,
	agentValue *protocol.Agent,
) (string, string, error) {
	if s.prefs == nil {
		return "", "", nil
	}
	ownerUserID := ""
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok {
		ownerUserID = currentUserID
	}
	if ownerUserID == "" && agentValue != nil {
		ownerUserID = strings.TrimSpace(agentValue.OwnerUserID)
	}
	if ownerUserID == "" {
		return "", "", nil
	}
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return "", "", err
	}
	provider := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
	model := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	if provider == "" || model == "" {
		return "", "", nil
	}
	return provider, model, nil
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
