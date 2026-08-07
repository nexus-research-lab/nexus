package slashcommand

import (
	"context"
	"strings"
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

func TestModelCommandRejectsMainAgent(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{IsMain: true, OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers:   fakeModelCommandProviders{options: &providersvc.OptionsResponse{}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{AgentID: "agent-main", Content: "/model deepseek/deepseek-v4-flash"},
	)
	message, clientSafe := protocol.ClientErrorMessage(err)
	if !matched || !clientSafe || !strings.Contains(message, "始终跟随") || sessions.updateCount != 0 {
		t.Fatalf(
			"Execute() = matched:%t err:%v client_safe:%t message:%q updates:%d",
			matched,
			err,
			clientSafe,
			message,
			sessions.updateCount,
		)
	}
}

func TestModelCommandResolvesUniqueUnqualifiedModel(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{{
				Provider: "deepseek",
				Models: []providersvc.ModelOption{{
					ModelID: "deepseek-v4-flash",
				}},
			}},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID:    "agent-a",
			Content:    "/model deepseek-v4-flash",
			SessionKey: "agent:agent-a:ws:dm:one",
		},
	)
	if err != nil || !matched ||
		sessions.settings.Provider != "deepseek" ||
		sessions.settings.Model != "deepseek-v4-flash" {
		t.Fatalf(
			"Execute() = matched:%t err:%v settings:%#v",
			matched,
			err,
			sessions.settings,
		)
	}
}

func TestModelCommandPreservesSlashInsideModelID(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{{
				Provider: "openrouter",
				Models: []providersvc.ModelOption{{
					ModelID: "anthropic/claude-sonnet-4",
				}},
			}},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID:    "agent-a",
			Content:    "/model openrouter/anthropic/claude-sonnet-4",
			SessionKey: "agent:agent-a:ws:dm:one",
		},
	)
	if err != nil || !matched ||
		sessions.settings.Provider != "openrouter" ||
		sessions.settings.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf(
			"Execute() = matched:%t err:%v settings:%#v",
			matched,
			err,
			sessions.settings,
		)
	}
}

func TestModelCommandResolvesUnqualifiedModelIDContainingSlash(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{{
				Provider: "openrouter",
				Models: []providersvc.ModelOption{{
					ModelID: "anthropic/claude-sonnet-4",
				}},
			}},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID:    "agent-a",
			Content:    "/model anthropic/claude-sonnet-4",
			SessionKey: "agent:agent-a:ws:dm:one",
		},
	)
	if err != nil || !matched ||
		sessions.settings.Provider != "openrouter" ||
		sessions.settings.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf(
			"Execute() = matched:%t err:%v settings:%#v",
			matched,
			err,
			sessions.settings,
		)
	}
}

func TestModelCommandRejectsAmbiguousModel(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{
				{
					Provider: "provider-a",
					Models:   []providersvc.ModelOption{{ModelID: "shared-model"}},
				},
				{
					Provider: "provider-b",
					Models:   []providersvc.ModelOption{{ModelID: "shared-model"}},
				},
			},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID: "agent-a",
			Content: "/model shared-model",
		},
	)
	message, clientSafe := protocol.ClientErrorMessage(err)
	if !matched || !clientSafe || message == "" || sessions.updateCount != 0 {
		t.Fatalf(
			"Execute() = matched:%t err:%v client_safe:%t updates:%d",
			matched,
			err,
			clientSafe,
			sessions.updateCount,
		)
	}
}

func TestModelCommandRejectsUnknownClaudeNativeAliasAtHost(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "claude"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeDM,
		Invocation{
			AgentID: "agent-a",
			Content: "/model sonnet",
		},
	)
	message, clientSafe := protocol.ClientErrorMessage(err)
	if !matched || !clientSafe || message == "" || sessions.updateCount != 0 {
		t.Fatalf(
			"Execute() = matched:%t err:%v client_safe:%t updates:%d",
			matched,
			err,
			clientSafe,
			sessions.updateCount,
		)
	}
}

func TestModelCommandTargetsSelectedRoomAgentSession(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	sessions := &fakeModelCommandSessions{}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
		Sessions:    sessions,
		Preferences: fakeModelCommandPreferences{runtimeKind: "nxs"},
		Providers: fakeModelCommandProviders{options: &providersvc.OptionsResponse{
			Items: []providersvc.Option{{
				Provider: "deepseek",
				Models: []providersvc.ModelOption{{
					ModelID: "deepseek-v4-flash",
				}},
			}},
		}},
	}); err != nil {
		t.Fatalf("RegisterModelCommand() error = %v", err)
	}

	_, matched, err := registry.Execute(
		context.Background(),
		ScopeRoom,
		Invocation{
			AgentID:    "agent-b",
			Content:    "/model deepseek-v4-flash",
			SessionKey: "room:group:conversation-a",
		},
	)
	if err != nil || !matched {
		t.Fatalf("Execute() = matched:%t err:%v", matched, err)
	}
	want := protocol.BuildRoomAgentSessionKey(
		"conversation-a",
		"agent-b",
		protocol.RoomTypeGroup,
	)
	if sessions.targetSessionKey != want {
		t.Fatalf("target session key = %q, want %q", sessions.targetSessionKey, want)
	}
}
