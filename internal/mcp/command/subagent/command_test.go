package subagent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func testActor() command.Actor {
	return command.Actor{OwnerUserID: "owner", AgentID: "agent", SessionKey: "session", RoundID: "round", LeaseSessionKey: "session", LeaseRoundID: "round", SourceContextType: "agent"}
}
func TestSubagentContractAndRequestDeduplication(t *testing.T) {
	var calls atomic.Int32
	h := NewHandler(testActor(), func(_ context.Context, id, op string, input map[string]any) (map[string]any, error) {
		calls.Add(1)
		if id != "sdk-call" || op != "spawn" {
			t.Fatalf("identity %q %q", id, op)
		}
		return map[string]any{"task_id": "child"}, nil
	})
	contract, err := h(context.Background(), command.Request{Action: "contract", Operation: "spawn"})
	if err != nil || len(contract.(command.Contract).Operations) != 1 || calls.Load() != 0 {
		t.Fatalf("contract: %#v %v", contract, err)
	}
	ctx := WithToolUseID(context.Background(), "sdk-call")
	request := command.Request{Action: "invoke", Operation: "spawn", RequestID: "request-1", Input: map[string]any{"description": "task", "prompt": "research"}}
	for range 2 {
		if _, err = h(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("spawn replayed %d times", calls.Load())
	}
	request.Input["prompt"] = "different"
	if _, err = h(ctx, request); err == nil {
		t.Fatal("conflicting intent accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("conflict reached runtime")
	}
}
func TestSubagentRejectsInvalidInputBeforeRuntime(t *testing.T) {
	var calls int
	h := NewHandler(testActor(), func(context.Context, string, string, map[string]any) (map[string]any, error) {
		calls++
		return nil, nil
	})
	for _, input := range []map[string]any{{"description": "task"}, {"description": "task", "prompt": " "}, {"description": "task", "prompt": "ok", "owner": "fake"}} {
		_, err := h(WithToolUseID(context.Background(), "sdk"), command.Request{Action: "invoke", Operation: "spawn", RequestID: "request-1", Input: input})
		if err == nil {
			t.Fatalf("accepted %#v", input)
		}
	}
	_, err := h(context.Background(), command.Request{Action: "invoke", Operation: "spawn", RequestID: "request-1", Input: map[string]any{"description": "task", "prompt": "ok"}})
	if err == nil || calls != 0 {
		t.Fatal("missing SDK identity reached runtime")
	}
}
func TestSubagentUnknownOutcomeIsNotReplayed(t *testing.T) {
	calls := 0
	h := NewHandler(testActor(), func(context.Context, string, string, map[string]any) (map[string]any, error) {
		calls++
		return nil, errors.New("transport lost after dispatch")
	})
	request := command.Request{Action: "invoke", Operation: "spawn", RequestID: "request-1", Input: map[string]any{"description": "task", "prompt": "research"}}
	for range 2 {
		if _, err := h(WithToolUseID(context.Background(), "sdk"), request); err == nil {
			t.Fatal("lost outcome reported success")
		}
	}
	if calls != 1 {
		t.Fatal("uncertain dispatch was replayed")
	}
}
