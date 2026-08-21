package runtimeselection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	clientopts "github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type fakePreferencesService struct {
	items map[string]preferencessvc.Preferences
}

func (s fakePreferencesService) Get(_ context.Context, ownerUserID string) (preferencessvc.Preferences, error) {
	return s.items[ownerUserID], nil
}

type fakeRuntimeConfigResolver func(context.Context, string, string, string) (*clientopts.RuntimeConfig, error)

func (resolver fakeRuntimeConfigResolver) ResolveRuntimeConfig(
	ctx context.Context,
	provider string,
	model string,
) (*clientopts.RuntimeConfig, error) {
	return resolver(ctx, provider, model, "")
}

func (resolver fakeRuntimeConfigResolver) ResolveRuntimeConfigForRuntime(
	ctx context.Context,
	provider string,
	model string,
	runtimeKind string,
) (*clientopts.RuntimeConfig, error) {
	return resolver(ctx, provider, model, runtimeKind)
}

func TestResolveUsesExplicitAgentModelAndPreferenceRuntimeKind(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind:           "nxs",
			AgentSDKDiagnosticsEnabled: true,
			EmotionEnabled:             true,
			DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
				Provider: "background-provider",
				Model:    "background-model",
			},
			RuntimeSettings: preferencessvc.RuntimeSettings{
				"nxs": {ToolSearch: true},
			},
			DefaultAgentOptions: protocol.Options{
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
	}})
	selection, err := service.Resolve(context.Background(), Request{
		Agent: &protocol.Agent{
			OwnerUserID: "owner-1",
			Options: protocol.Options{
				Provider: "anthropic",
				Model:    "claude-sonnet-4-5",
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.RuntimeKind != "nxs" || selection.Provider != "anthropic" || selection.Model != "claude-sonnet-4-5" {
		t.Fatalf("显式 Agent 模型应优先，同时保留偏好 runtime: %+v", selection)
	}
	if !selection.AgentSDKDiagnosticsEnabled {
		t.Fatalf("Agent SDK diagnostics 偏好应透传: %+v", selection)
	}
	if !selection.EmotionEnabled {
		t.Fatalf("情绪系统偏好应透传: %+v", selection)
	}
	if !selection.ToolSearchEnabled {
		t.Fatalf("nxs ToolSearch 偏好应透传: %+v", selection)
	}
	if selection.BackgroundProvider != "background-provider" ||
		selection.BackgroundModel != "background-model" {
		t.Fatalf("后台模型偏好应透传给 bridge: %+v", selection)
	}
}

func TestResolvePrefersSessionModelOverAgentAndGlobalDefaults(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind: "nxs",
			DefaultAgentOptions: protocol.Options{
				Provider: "global-provider",
				Model:    "global-model",
			},
		},
	}})
	selection, err := service.Resolve(context.Background(), Request{
		Agent: &protocol.Agent{
			OwnerUserID: "owner-1",
			Options: protocol.Options{
				Provider: "agent-provider",
				Model:    "agent-model",
			},
		},
		SessionOptions: protocol.WithSessionRuntimeSettings(
			nil,
			protocol.SessionRuntimeSettings{
				Provider: "session-provider",
				Model:    "session-model",
			},
		),
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.Provider != "session-provider" ||
		selection.Model != "session-model" {
		t.Fatalf("Session 模型覆盖优先级错误: %+v", selection)
	}
}

func TestResolveFallsBackToPreferenceDefaultModel(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind: "nxs",
			DefaultAgentOptions: protocol.Options{
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
	}})
	selection, err := service.Resolve(context.Background(), Request{
		Agent: &protocol.Agent{OwnerUserID: "owner-1"},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.RuntimeKind != "nxs" || selection.Provider != "openai" || selection.Model != "gpt-4o" {
		t.Fatalf("未显式配置模型时应使用用户默认模型: %+v", selection)
	}
	if selection.AgentSDKDiagnosticsEnabled {
		t.Fatalf("Agent SDK diagnostics 默认应保持关闭: %+v", selection)
	}
}

func TestResolveMainAgentAlwaysUsesPreferenceDefaultModel(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind: "nxs",
			DefaultAgentOptions: protocol.Options{
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
	}})
	selection, err := service.Resolve(context.Background(), Request{
		Agent: &protocol.Agent{
			IsMain:      true,
			OwnerUserID: "owner-1",
			Options: protocol.Options{
				Provider: "stale-provider",
				Model:    "stale-model",
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.Provider != "openai" || selection.Model != "gpt-4o" {
		t.Fatalf("主智能体不应使用历史显式模型: %+v", selection)
	}
}

func TestResolveTemporarilyFallsBackAndRestoresExplicitAgentModel(t *testing.T) {
	explicitAvailable := false
	resolver := fakeRuntimeConfigResolver(func(
		_ context.Context,
		provider string,
		model string,
		_ string,
	) (*clientopts.RuntimeConfig, error) {
		if provider == "saved-provider" && model == "saved-model" && !explicitAvailable {
			return nil, errors.New("saved provider unavailable")
		}
		if (provider == "saved-provider" && model == "saved-model") ||
			(provider == "default-provider" && model == "default-model") {
			return &clientopts.RuntimeConfig{Provider: provider, Model: model}, nil
		}
		return nil, errors.New("model unavailable")
	})
	service := NewServiceWithRuntimeConfigResolver(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind: "nxs",
			DefaultAgentOptions: protocol.Options{
				Provider: "default-provider",
				Model:    "default-model",
			},
		},
	}}, resolver)
	request := Request{Agent: &protocol.Agent{
		OwnerUserID: "owner-1",
		Options: protocol.Options{
			Provider: "saved-provider",
			Model:    "saved-model",
		},
	}}

	fallback, err := service.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve 临时回退失败: %v", err)
	}
	if !fallback.FallbackFromExplicit || fallback.Provider != "default-provider" || fallback.Model != "default-model" {
		t.Fatalf("显式模型不可用时应临时使用默认模型: %+v", fallback)
	}

	explicitAvailable = true
	restored, err := service.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve 恢复显式模型失败: %v", err)
	}
	if restored.FallbackFromExplicit || restored.Provider != "saved-provider" || restored.Model != "saved-model" {
		t.Fatalf("显式模型恢复后应自动重新生效: %+v", restored)
	}
}

func TestResolveNormalizesPreferenceRuntimeKind(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind: "GO-native",
		},
	}})
	selection, err := service.Resolve(context.Background(), Request{
		Agent: &protocol.Agent{OwnerUserID: "owner-1"},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.RuntimeKind != "nxs" {
		t.Fatalf("偏好 runtime 别名未归一化: %+v", selection)
	}
}

func TestResolvePrefersContextOwnerBeforeRequestOwners(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"context-owner": {
			AgentRuntimeKind: "nxs",
			DefaultAgentOptions: protocol.Options{
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
		"round-owner": {
			AgentRuntimeKind: "claude",
			DefaultAgentOptions: protocol.Options{
				Provider: "anthropic",
				Model:    "claude-sonnet-4-5",
			},
		},
	}})
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "context-owner"})
	selection, err := service.Resolve(ctx, Request{
		OwnerUserIDs: []string{"round-owner"},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.RuntimeKind != "nxs" || selection.Provider != "openai" || selection.Model != "gpt-4o" {
		t.Fatalf("当前用户上下文应优先于请求 owner: %+v", selection)
	}
}

func TestResolveKeepsPartialAgentModelWithoutPreferences(t *testing.T) {
	selection, err := NewService(nil).Resolve(context.Background(), Request{
		Agent: &protocol.Agent{
			Options: protocol.Options{
				Provider: "openai",
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if selection.RuntimeKind != "" || selection.Provider != "openai" || selection.Model != "" {
		t.Fatalf("没有偏好时应保持原有部分显式选择行为: %+v", selection)
	}
}

func TestWebSearchConfigFromPreferencesCopiesRuntimeSettings(t *testing.T) {
	settings := preferencessvc.WebSearchSettings{
		Enabled:             true,
		Provider:            "brave",
		BaseURL:             "https://search.example.com",
		AllowPrivateNetwork: true,
		UseProviderExtract:  true,
		DefaultCount:        8,
		TimeoutSeconds:      20,
		CacheTTLSeconds:     120,
		Country:             "CN",
		Language:            "zh-CN",
		SearchLanguage:      "zh",
		Freshness:           "week",
		SearchDepth:         "advanced",
		ExtractDepth:        "deep",
		AnySearch: preferencessvc.AnySearchSettings{
			Domain:       "example.com",
			Tag:          "docs",
			ContentTypes: []string{"article"},
			Params:       map[string]any{"limit": 4},
		},
	}.WithWebSearchAPIKey("search-key")

	config := WebSearchConfigFromPreferences(settings)
	settings.AnySearch.ContentTypes[0] = "changed"
	settings.AnySearch.Params["limit"] = 9

	if !config.Enabled || config.Provider != "brave" {
		t.Fatalf("WebSearch 主配置投影错误: %+v", config)
	}
	if env := clientopts.BuildWebSearchRuntimeEnv("nxs", config); env["NEXUS_WEBSEARCH_API_KEY"] != "search-key" {
		t.Fatalf("WebSearch 密钥投影错误: %+v", env)
	}
	rawConfig, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("序列化 WebSearch runtime 配置失败: %v", err)
	}
	if strings.Contains(string(rawConfig), "search-key") {
		t.Fatalf("WebSearch runtime 配置泄漏了原始密钥: %s", rawConfig)
	}
	if config.AnySearch.ContentTypes[0] != "article" || config.AnySearch.Params["limit"] != 4 {
		t.Fatalf("AnySearch 配置必须与持久化对象隔离: %+v", config.AnySearch)
	}
}
