// slashcommand_test.go 覆盖 Slash 命令注册表、模型、Goal 与计划命令测试。
package slashcommand

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type fakeModelCommandAgents struct {
	agent protocol.Agent
}

func (f *fakeModelCommandAgents) GetAgent(
	_ context.Context,
	agentID string,
) (*protocol.Agent, error) {
	result := f.agent
	result.AgentID = agentID
	return &result, nil
}

type fakeModelCommandSessions struct {
	settings         protocol.SessionRuntimeSettings
	targetSessionKey string
	updateCount      int
}

func (f *fakeModelCommandSessions) GetRuntimeSettings(
	_ context.Context,
	sessionKey string,
) (protocol.SessionRuntimeSettings, error) {
	f.targetSessionKey = sessionKey
	return f.settings, nil
}

func (f *fakeModelCommandSessions) UpdateRuntimeSettings(
	_ context.Context,
	sessionKey string,
	settings protocol.SessionRuntimeSettings,
) (protocol.SessionRuntimeSettings, error) {
	f.targetSessionKey = sessionKey
	f.settings = settings
	f.updateCount++
	return settings, nil
}

type fakeModelCommandPreferences struct {
	runtimeKind string
}

func (f fakeModelCommandPreferences) Get(
	context.Context,
	string,
) (preferencessvc.Preferences, error) {
	return preferencessvc.Preferences{
		AgentRuntimeKind: f.runtimeKind,
	}, nil
}

type fakeModelCommandProviders struct {
	options *providersvc.OptionsResponse
}

func (f fakeModelCommandProviders) ListOptionsForRuntime(
	context.Context,
	string,
) (*providersvc.OptionsResponse, error) {
	return f.options, nil
}

func TestModelCommandPersistsQualifiedProviderSelection(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{
		settings: protocol.SessionRuntimeSettings{PermissionMode: "acceptEdits"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{{
				Provider:    "deepseek",
				DisplayName: "DeepSeek",
				Models: []providersvc.ModelOption{{
					ModelID:     "deepseek-v4-flash",
					DisplayName: "DeepSeek V4 Flash",
				}},
			}},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	result, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID:    "agent-a",
			Content:    "/model deepseek/deepseek-v4-flash",
			RoundID:    "round-a",
			SessionKey: "agent:agent-a:ws:dm:one",
		},
	)
	if err != nil || !matched {
		t.Fatalf("Execute() = matched:%t err:%v", matched, err)
	}
	if sessions.updateCount != 1 ||
		sessions.targetSessionKey != "agent:agent-a:ws:dm:one" ||
		sessions.settings.Provider != "deepseek" ||
		sessions.settings.Model != "deepseek-v4-flash" ||
		sessions.settings.PermissionMode != "acceptEdits" {
		t.Fatalf("session update = %#v count=%d", sessions, sessions.updateCount)
	}
	if result.DirectoryInvalidation != nil || len(result.Events) != 1 {
		t.Fatalf("result = %#v", result)
	}
	event := result.Events[0]
	if event.EventType != protocol.EventTypeMessage ||
		event.DeliveryMode != protocol.DeliveryModeTransient ||
		event.RoundID != "round-a" ||
		event.Data["stop_reason"] != "end_turn" {
		t.Fatalf("model changed event = %#v", event)
	}
}
