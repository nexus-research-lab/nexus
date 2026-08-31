// INPUT: Agent、session 覆盖项与用户 runtime 偏好。
// OUTPUT: 已归一化的 runtime、主/后台/视觉模型和能力启动配置。
// POS: handler/业务编排与底层 runtime clientopts 之间的选择和投影边界。
package runtimeselection

import (
	"cmp"
	"context"
	"fmt"
	"maps"
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
	BackgroundProvider         string
	BackgroundModel            string
	VisionProvider             string
	VisionModel                string
	AgentSDKDiagnosticsEnabled bool
	EmotionEnabled             bool
	AutoMemoryDisabled         bool
	AutoDreamDisabled          bool
	ToolSearchEnabled          bool
	WebSearch                  clientopts.WebSearchConfig
}

// Request 表示一次 Agent runtime 选择请求。
type Request struct {
	Agent          *protocol.Agent
	OwnerUserIDs   []string
	SessionOptions map[string]any
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

// Resolve 依次合并 Session 覆盖、普通 Agent 显式模型与用户全局默认；Nexus 主智能体不读取历史 Agent 模型。
func (s *Service) Resolve(ctx context.Context, request Request) (Selection, error) {
	selection := Selection{}
	sessionProvider, sessionModel := explicitSessionModel(request.SessionOptions)
	agentProvider, agentModel := explicitAgentModel(request.Agent)
	hasExplicitSessionModel := sessionProvider != "" && sessionModel != ""
	hasExplicitAgentModel := !hasExplicitSessionModel && agentProvider != "" && agentModel != ""
	switch {
	case hasExplicitSessionModel:
		selection.Provider = sessionProvider
		selection.Model = sessionModel
	case hasExplicitAgentModel:
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
		selection.EmotionEnabled = prefs.EmotionEnabled
		selection.AutoMemoryDisabled = !prefs.AutoMemoryEnabledForRuntime(runtimeprovider.RuntimeKindNXS)
		selection.AutoDreamDisabled = !prefs.AutoDreamEnabledForRuntime(runtimeprovider.RuntimeKindNXS)
		selection.ToolSearchEnabled = prefs.ToolSearchEnabledForRuntime(selection.RuntimeKind)
		selection.WebSearch = WebSearchConfigFromPreferences(prefs.WebSearch)
		selection.BackgroundProvider = strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Provider)
		selection.BackgroundModel = strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Model)
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

// WebSearchConfigFromPreferences 将持久化偏好投影为 runtime 启动配置。
func WebSearchConfigFromPreferences(settings preferencessvc.WebSearchSettings) clientopts.WebSearchConfig {
	config := clientopts.WebSearchConfig{
		Enabled:             settings.Enabled,
		Provider:            settings.Provider,
		BaseURL:             settings.BaseURL,
		AllowPrivateNetwork: settings.AllowPrivateNetwork,
		UseProviderExtract:  settings.UseProviderExtract,
		DefaultCount:        settings.DefaultCount,
		TimeoutSeconds:      settings.TimeoutSeconds,
		CacheTTLSeconds:     settings.CacheTTLSeconds,
		Country:             settings.Country,
		Language:            settings.Language,
		SearchLanguage:      settings.SearchLanguage,
		Freshness:           settings.Freshness,
		SearchDepth:         settings.SearchDepth,
		ExtractDepth:        settings.ExtractDepth,
		AnySearch: clientopts.AnySearchConfig{
			Domain:       settings.AnySearch.Domain,
			Tag:          settings.AnySearch.Tag,
			ContentTypes: append([]string(nil), settings.AnySearch.ContentTypes...),
			Params:       maps.Clone(settings.AnySearch.Params),
		},
	}
	return config.WithAPIKey(settings.WebSearchAPIKey())
}

// RuntimeEnvironmentFromPreferences 把可热更新的用户运行偏好投影为 nxs 环境。
func RuntimeEnvironmentFromPreferences(preferences preferencessvc.Preferences) map[string]string {
	environment := clientopts.BuildWebSearchRuntimeEnv(
		runtimeprovider.RuntimeKindNXS,
		WebSearchConfigFromPreferences(preferences.WebSearch),
	)
	maps.Copy(environment, clientopts.BuildAutoMemoryRuntimeEnv(
		runtimeprovider.RuntimeKindNXS,
		!preferences.AutoMemoryEnabledForRuntime(runtimeprovider.RuntimeKindNXS),
	))
	maps.Copy(environment, clientopts.BuildAutoDreamRuntimeEnv(
		runtimeprovider.RuntimeKindNXS,
		!preferences.AutoDreamEnabledForRuntime(runtimeprovider.RuntimeKindNXS),
	))
	return environment
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

func explicitSessionModel(options map[string]any) (string, string) {
	settings := protocol.SessionRuntimeSettingsFromOptions(options)
	if settings.Provider == "" || settings.Model == "" {
		return "", ""
	}
	return settings.Provider, settings.Model
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
