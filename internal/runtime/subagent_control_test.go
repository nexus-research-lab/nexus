package runtime

import (
	"context"
	"testing"
)

type subagentControlClient struct {
	fakeRuntimeClient
	calls int
}

func (c *subagentControlClient) ControlSubagent(_ context.Context, id, operation string, _ map[string]any) (map[string]any, error) {
	c.calls++
	return map[string]any{"id": id, "operation": operation}, nil
}
func TestSubagentControlCannotBorrowSuccessorRound(t *testing.T) {
	manager := NewManager()
	client := &subagentControlClient{}
	manager.sessions["parent"] = &sessionState{Client: client, SubagentHooks: map[string]SubagentHookCallbacks{"round-1": {}}}
	control := manager.BindSubagentControl("parent", "round-1")
	if _, err := control(context.Background(), "tool-1", "list", nil); err != nil {
		t.Fatal(err)
	}
	delete(manager.sessions["parent"].SubagentHooks, "round-1")
	manager.sessions["parent"].SubagentHooks["round-2"] = SubagentHookCallbacks{}
	if _, err := control(context.Background(), "tool-2", "spawn", nil); err == nil {
		t.Fatal("old round borrowed successor")
	}
	if client.calls != 1 {
		t.Fatal("expired round reached runtime")
	}
	if _, err := manager.BindSubagentControl("other-parent", "round-2")(context.Background(), "tool-3", "list", nil); err == nil {
		t.Fatal("foreign session accepted")
	}
}
