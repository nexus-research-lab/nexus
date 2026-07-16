package runtimeselection

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type fakePreferencesService struct {
	items map[string]preferencessvc.Preferences
}

func (s fakePreferencesService) Get(_ context.Context, ownerUserID string) (preferencessvc.Preferences, error) {
	return s.items[ownerUserID], nil
}

func TestResolveUsesExplicitAgentModelAndPreferenceRuntimeKind(t *testing.T) {
	service := NewService(fakePreferencesService{items: map[string]preferencessvc.Preferences{
		"owner-1": {
			AgentRuntimeKind:           "nxs",
			AgentSDKDiagnosticsEnabled: true,
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
	if !selection.ToolSearchEnabled {
		t.Fatalf("nxs ToolSearch 偏好应透传: %+v", selection)
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
