package slashcommand

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type fakeModelCommandAgents struct {
	agent       protocol.Agent
	updateCount int
}

func (f *fakeModelCommandAgents) GetAgent(
	_ context.Context,
	agentID string,
) (*protocol.Agent, error) {
	result := f.agent
	result.AgentID = agentID
	return &result, nil
}

func (f *fakeModelCommandAgents) UpdateAgent(
	_ context.Context,
	agentID string,
	request protocol.UpdateRequest,
) (*protocol.Agent, error) {
	f.updateCount++
	f.agent.AgentID = agentID
	if request.Options != nil {
		f.agent.Options.Provider = request.Options.Provider
		f.agent.Options.Model = request.Options.Model
	}
	result := f.agent
	return &result, nil
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
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
	if agents.updateCount != 1 ||
		agents.agent.Options.Provider != "deepseek" ||
		agents.agent.Options.Model != "deepseek-v4-flash" {
		t.Fatalf("agent update = %#v count=%d", agents.agent.Options, agents.updateCount)
	}
	if result.DirectoryInvalidation == nil ||
		result.DirectoryInvalidation.Reason != "agent_updated" ||
		len(result.Events) != 1 {
		t.Fatalf("result = %#v", result)
	}
	event := result.Events[0]
	if event.EventType != protocol.EventTypeMessage ||
		event.DeliveryMode != "ephemeral" ||
		event.RoundID != "round-a" ||
		event.Data["stop_reason"] != "end_turn" {
		t.Fatalf("model changed event = %#v", event)
	}
}

func TestModelCommandResolvesUniqueUnqualifiedModel(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
			AgentID: "agent-a",
			Content: "/model deepseek-v4-flash",
		},
	)
	if err != nil || !matched ||
		agents.agent.Options.Provider != "deepseek" ||
		agents.agent.Options.Model != "deepseek-v4-flash" {
		t.Fatalf(
			"Execute() = matched:%t err:%v options:%#v",
			matched,
			err,
			agents.agent.Options,
		)
	}
}

func TestModelCommandPreservesSlashInsideModelID(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
			AgentID: "agent-a",
			Content: "/model openrouter/anthropic/claude-sonnet-4",
		},
	)
	if err != nil || !matched ||
		agents.agent.Options.Provider != "openrouter" ||
		agents.agent.Options.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf(
			"Execute() = matched:%t err:%v options:%#v",
			matched,
			err,
			agents.agent.Options,
		)
	}
}

func TestModelCommandResolvesUnqualifiedModelIDContainingSlash(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
			AgentID: "agent-a",
			Content: "/model anthropic/claude-sonnet-4",
		},
	)
	if err != nil || !matched ||
		agents.agent.Options.Provider != "openrouter" ||
		agents.agent.Options.Model != "anthropic/claude-sonnet-4" {
		t.Fatalf(
			"Execute() = matched:%t err:%v options:%#v",
			matched,
			err,
			agents.agent.Options,
		)
	}
}

func TestModelCommandRejectsAmbiguousModel(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
	if !matched || !clientSafe || message == "" || agents.updateCount != 0 {
		t.Fatalf(
			"Execute() = matched:%t err:%v client_safe:%t updates:%d",
			matched,
			err,
			clientSafe,
			agents.updateCount,
		)
	}
}

func TestModelCommandRejectsUnknownClaudeNativeAliasAtHost(t *testing.T) {
	agents := &fakeModelCommandAgents{
		agent: protocol.Agent{OwnerUserID: "owner-a"},
	}
	registry := NewRegistry()
	if err := RegisterModelCommand(registry, ModelCommandDependencies{
		Agents:      agents,
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
	if !matched || !clientSafe || message == "" || agents.updateCount != 0 {
		t.Fatalf(
			"Execute() = matched:%t err:%v client_safe:%t updates:%d",
			matched,
			err,
			clientSafe,
			agents.updateCount,
		)
	}
}
