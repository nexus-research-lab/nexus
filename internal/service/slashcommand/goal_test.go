package slashcommand

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestGoalCommandUsesOneExecutorForSlashAndStructuredOptions(t *testing.T) {
	executor := &goalCommandExecutorSpy{}
	registry := NewRegistry()
	if err := RegisterGoalCommand(registry, GoalCommandDependencies{Executor: executor}); err != nil {
		t.Fatalf("RegisterGoalCommand() error = %v", err)
	}
	replace := true
	result, matched, err := registry.Execute(context.Background(), ScopeRoom, Invocation{
		SessionKey:      "room:group:conversation-1",
		RoundID:         "round-goal",
		UserMessageID:   "message-goal",
		ClientRequestID: "request-goal",
		ClientMessageID: "client-goal",
		Content:         "/goal Ship the release",
		TargetAgentIDs:  []string{"agent-lead"},
		GoalOptions: protocol.GoalCommandOptions{
			ReplaceExisting: &replace,
			Metadata:        map[string]any{"room_goal_loop_slug": "release"},
		},
	})
	if err != nil || !matched {
		t.Fatalf("Execute() matched=%v error=%v", matched, err)
	}
	if !result.UserMessageCommitted || result.AfterResponseAttempted == nil {
		t.Fatalf("result = %#v, want durable ACK and deferred continuation", result)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("requests = %#v", executor.requests)
	}
	request := executor.requests[0]
	if request.Objective != "Ship the release" ||
		request.CommandContent != "/goal Ship the release" ||
		len(request.TargetAgentIDs) != 1 ||
		request.TargetAgentIDs[0] != "agent-lead" {
		t.Fatalf("request = %#v", request)
	}
	if len(executor.dispatched) != 0 {
		t.Fatal("continuation dispatched before ACK callback")
	}
	result.AfterResponseAttempted(context.Background())
	if len(executor.dispatched) != 1 || executor.dispatched[0].ID != "goal-command" {
		t.Fatalf("dispatched = %#v", executor.dispatched)
	}
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
