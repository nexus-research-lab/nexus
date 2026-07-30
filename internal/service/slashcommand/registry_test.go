package slashcommand

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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
