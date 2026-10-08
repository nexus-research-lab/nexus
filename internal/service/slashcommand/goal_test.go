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
