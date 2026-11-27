// slashcommand_test.go 覆盖 Slash 命令注册表、模型、Goal 与计划命令测试。
package slashcommand

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestRegistrySeparatesHostCommandsByScope(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Definition{
		Name:        "goal",
		Description: "Set a Nexus goal",
		Scopes:      []Scope{ScopeDM},
		Enabled:     true,
		Handler: func(context.Context, Invocation) (Result, error) {
			return Result{}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	dmCommands := registry.Descriptors(ScopeDM)
	if len(dmCommands) != 1 ||
		dmCommands[0].Name != "goal" ||
		dmCommands[0].Execution != protocol.CommandExecutionHost {
		t.Fatalf("DM descriptors = %#v, want host goal command", dmCommands)
	}
	if roomCommands := registry.Descriptors(ScopeRoom); len(roomCommands) != 0 {
		t.Fatalf("Room descriptors = %#v, want empty", roomCommands)
	}
}

func TestRegistryExecutesKnownHostCommandAndLeavesUnknownForRuntime(t *testing.T) {
	registry := NewRegistry()
	var received Invocation
	if err := registry.Register(Definition{
		Name:    "goal",
		Scopes:  []Scope{ScopeDM},
		Enabled: true,
		Handler: func(_ context.Context, invocation Invocation) (Result, error) {
			received = invocation
			return Result{}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, matched, err := registry.Execute(context.Background(), ScopeDM, Invocation{
		SessionKey: "agent:a:ws:dm:one",
		Content:    "/goal ship the release",
	})
	if err != nil || !matched || received.Arguments != "ship the release" {
		t.Fatalf("Execute() = matched:%t err:%v invocation:%#v", matched, err, received)
	}
	if _, matched, err = registry.Execute(context.Background(), ScopeDM, Invocation{
		Content: "/review target",
	}); err != nil || matched {
		t.Fatalf("unknown runtime command matched host registry: matched=%t err=%v", matched, err)
	}
}

func TestRegistryRejectsAttachmentsBeforeHostHandler(t *testing.T) {
	registry := NewRegistry()
	called := false
	if err := registry.Register(Definition{
		Name:    "goal",
		Scopes:  []Scope{ScopeDM},
		Enabled: true,
		Handler: func(context.Context, Invocation) (Result, error) {
			called = true
			return Result{}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, matched, err := registry.Execute(context.Background(), ScopeDM, Invocation{
		Content:         "/goal ship",
		AttachmentCount: 1,
	})
	message, clientSafe := protocol.ClientErrorMessage(err)
	if !matched || called || !clientSafe || message == "" {
		t.Fatalf(
			"Execute() = matched:%t called:%t err:%v client_message:%q",
			matched,
			called,
			err,
			message,
		)
	}
}

func TestRegistryAuthorizesKnownHostCommandBeforeHandler(t *testing.T) {
	registry := NewRegistry()
	called := false
	if err := registry.Register(Definition{
		Name:    "goal",
		Scopes:  []Scope{ScopeDM},
		Enabled: true,
		Handler: func(context.Context, Invocation) (Result, error) {
			called = true
			return Result{}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, matched, err := registry.ExecuteAuthorized(
		context.Background(),
		ScopeDM,
		Invocation{Content: "/goal ship", AttachmentCount: 1},
		func(_ context.Context, invocation Invocation) error {
			if invocation.Arguments != "ship" {
				t.Fatalf("authorized invocation arguments = %q, want ship", invocation.Arguments)
			}
			return errors.New("not authorized")
		},
	)
	if !matched || called || err == nil || err.Error() != "not authorized" {
		t.Fatalf(
			"ExecuteAuthorized() = matched:%t called:%t err:%v, want authorization failure",
			matched,
			called,
			err,
		)
	}
}

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

type goalCommandExecutorSpy struct {
	requests   []protocol.GoalCommandRequest
	dispatched []protocol.Goal
}

func (s *goalCommandExecutorSpy) ExecuteGoalCommand(
	_ context.Context,
	request protocol.GoalCommandRequest,
) (protocol.GoalCommandResult, error) {
	s.requests = append(s.requests, request)
	return protocol.GoalCommandResult{
		Goal: protocol.Goal{
			ID:         "goal-command",
			SessionKey: request.SessionKey,
			Objective:  request.Objective,
			Status:     protocol.GoalStatusActive,
		},
		UserMessageCommitted: true,
	}, nil
}

func (s *goalCommandExecutorSpy) DispatchGoalContinuation(_ context.Context, item protocol.Goal) {
	s.dispatched = append(s.dispatched, item)
}

func TestGoalCommandRejectsMissingObjectiveWithoutCallingExecutor(t *testing.T) {
	executor := &goalCommandExecutorSpy{}
	registry := NewRegistry()
	if err := RegisterGoalCommand(registry, GoalCommandDependencies{Executor: executor}); err != nil {
		t.Fatalf("RegisterGoalCommand() error = %v", err)
	}
	_, matched, err := registry.Execute(context.Background(), ScopeDM, Invocation{Content: "/goal"})
	if !matched || err == nil {
		t.Fatalf("Execute() matched=%v error=%v, want usage error", matched, err)
	}
	if len(executor.requests) != 0 {
		t.Fatalf("requests = %#v, want none", executor.requests)
	}
}

func TestPlanCommandRequiresExactExplicitInput(t *testing.T) {
	for _, tc := range []struct {
		content        string
		internal, plan bool
	}{
		{"/plan", false, true}, {" /PLAN design search", false, true},
		{"/plan design search", true, false}, {"please /plan", false, false},
		{"/planning", false, false}, {"/compact", false, false},
	} {
		got := PlanRequestPermissionMode(tc.content, tc.internal, sdkpermission.ModeBypassPermissions)
		want := sdkpermission.ModeBypassPermissions
		if tc.plan {
			want = sdkpermission.ModePlan
		}
		if got != want {
			t.Errorf("%q internal=%t: got %q want %q", tc.content, tc.internal, got, want)
		}
	}
	for _, content := range []string{"/plan", "/plan design search"} {
		prompt := ExpandProductPrompt(content)
		if !strings.Contains(prompt, "ExitPlanMode") || !strings.Contains(prompt, "before approval") || strings.HasPrefix(prompt, "/plan") {
			t.Fatalf("plan did not expand into the approval workflow: %q", prompt)
		}
	}
}
