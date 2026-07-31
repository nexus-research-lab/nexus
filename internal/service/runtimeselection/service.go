package runtimeselection

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	clientopts "github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

// PreferencesService 提供用户级 runtime 默认值读取能力。
type PreferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

// Service 收口 Agent runtime 的最终选择逻辑。
type Service struct {
	prefs          PreferencesService
	providerConfig clientopts.RuntimeConfigResolver
}

// Selection 表示启动 runtime 前已经合并完成的选择。
type Selection struct {
	RuntimeKind                string
	Provider                   string
	Model                      string
	FallbackFromExplicit       bool
	VisionProvider             string
	VisionModel                string
	AgentSDKDiagnosticsEnabled bool
	ToolSearchEnabled          bool
	WebSearch                  preferencessvc.WebSearchSettings
}

// Request 表示一次 Agent runtime 选择请求。
type Request struct {
	Agent        *protocol.Agent
	OwnerUserIDs []string
}

// NewService 创建 runtime 选择服务。
func NewService(prefs PreferencesService) *Service {
	return NewServiceWithRuntimeConfigResolver(prefs, nil)
}

// NewServiceWithRuntimeConfigResolver 创建会在 Agent 显式模型暂不可用时回退到默认模型的选择服务。
func NewServiceWithRuntimeConfigResolver(
	prefs PreferencesService,
	providerConfig clientopts.RuntimeConfigResolver,
) *Service {
	return &Service{prefs: prefs, providerConfig: providerConfig}
}

// Resolve 以普通 Agent 的显式模型优先；Nexus 主智能体始终回退到用户偏好中的默认 runtime/provider/model。
func (s *Service) Resolve(ctx context.Context, request Request) (Selection, error) {
	selection := Selection{}
	agentProvider, agentModel := explicitAgentModel(request.Agent)
	hasExplicitAgentModel := agentProvider != "" && agentModel != ""
	if hasExplicitAgentModel {
		selection.Provider = agentProvider
		selection.Model = agentModel
	}

	prefs, ok, err := s.preferences(ctx, request)
	if err != nil {
		return Selection{}, err
	}
	if ok {
		selection.RuntimeKind = runtimeprovider.NormalizeRuntimeKind(prefs.AgentRuntimeKind)
		selection.AgentSDKDiagnosticsEnabled = prefs.AgentSDKDiagnosticsEnabled
		selection.ToolSearchEnabled = prefs.ToolSearchEnabledForRuntime(selection.RuntimeKind)
		selection.WebSearch = prefs.WebSearch
		selection.VisionProvider = strings.TrimSpace(prefs.DefaultVisionModelSelection.Provider)
		selection.VisionModel = strings.TrimSpace(prefs.DefaultVisionModelSelection.Model)
		if selection.Provider == "" || selection.Model == "" {
			defaultProvider := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
			defaultModel := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
			if defaultProvider != "" && defaultModel != "" {
				selection.Provider = defaultProvider
				selection.Model = defaultModel
			}
		}
	}
	if !hasExplicitAgentModel && (selection.Provider == "" || selection.Model == "") {
		selection.Provider = cmp.Or(strings.TrimSpace(selection.Provider), agentProvider)
		selection.Model = cmp.Or(strings.TrimSpace(selection.Model), agentModel)
	}
	if !hasExplicitAgentModel || s == nil || s.providerConfig == nil {
		return selection, nil
	}
	if _, err = s.resolveRuntimeConfig(ctx, agentProvider, agentModel, selection.RuntimeKind); err == nil {
		return selection, nil
	}

	fallbackProvider, fallbackModel := "", ""
	if ok {
		fallbackProvider = strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
		fallbackModel = strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	}
	fallback, fallbackErr := s.resolveRuntimeConfig(
		ctx,
		fallbackProvider,
		fallbackModel,
		selection.RuntimeKind,
	)
	if fallbackErr != nil || fallback == nil {
		if fallbackErr == nil {
			fallbackErr = fmt.Errorf("默认模型解析结果为空")
		}
		return Selection{}, fmt.Errorf("Agent 显式模型不可用，且默认模型不可用: %w", fallbackErr)
	}
	selection.Provider = strings.TrimSpace(fallback.Provider)
	selection.Model = strings.TrimSpace(fallback.Model)
	selection.FallbackFromExplicit = true
	return selection, nil
}

func (s *Service) resolveRuntimeConfig(
	ctx context.Context,
	provider string,
	model string,
	runtimeKind string,
) (*clientopts.RuntimeConfig, error) {
	if resolver, ok := s.providerConfig.(clientopts.RuntimeConfigForRuntimeResolver); ok {
		return resolver.ResolveRuntimeConfigForRuntime(ctx, provider, model, runtimeKind)
	}
	return s.providerConfig.ResolveRuntimeConfig(ctx, provider, model)
}

func (s *Service) preferences(
	ctx context.Context,
	request Request,
) (preferencessvc.Preferences, bool, error) {
	if s == nil || s.prefs == nil {
		return preferencessvc.Preferences{}, false, nil
	}
	ownerUserID := ownerUserIDFromRequest(ctx, request)
	if ownerUserID == "" {
		return preferencessvc.Preferences{}, false, nil
	}
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return preferencessvc.Preferences{}, false, err
	}
	return prefs, true, nil
}

func ownerUserIDFromRequest(ctx context.Context, request Request) string {
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok {
		if ownerUserID := strings.TrimSpace(currentUserID); ownerUserID != "" {
			return ownerUserID
		}
	}
	for _, candidate := range request.OwnerUserIDs {
		if ownerUserID := strings.TrimSpace(candidate); ownerUserID != "" {
			return ownerUserID
		}
	}
	if request.Agent != nil {
		return strings.TrimSpace(request.Agent.OwnerUserID)
	}
	return ""
}

func explicitAgentModel(agent *protocol.Agent) (string, string) {
	if agent == nil || agent.IsMain {
		return "", ""
	}
	return strings.TrimSpace(agent.Options.Provider), strings.TrimSpace(agent.Options.Model)
}
