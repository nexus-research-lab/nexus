package tool

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

func TestBuildAllExposesCodexGoalToolSet(t *testing.T) {
	tools := BuildAll(nil, contract.ServerContext{CurrentSessionKey: "agent:nexus:ws:dm:chat"})
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, item.Name)
	}

	want := []string{"get_goal", "create_goal", "update_goal"}
	if !slices.Equal(names, want) {
		t.Fatalf("tool names = %#v, want %#v", names, want)
	}
}
