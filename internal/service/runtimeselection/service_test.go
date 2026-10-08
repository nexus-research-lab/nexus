package runtimeselection

import (
	"context"
	"errors"
	"testing"

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
