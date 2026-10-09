package operation

import (
	"context"
	"strings"
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
)

func TestPlanTransportRetryGuardSurvivesOperationRegistryRebuild(t *testing.T) {
	attempts := nexusmcp.NewCommandAttemptState()
	sctx := contract.Context{CommandAttempts: attempts}
	invokeEmptyPrepare := func() command.Result {
		definition, ok := command.FindOperation(BuildAll(nil, sctx), "prepare_plan_execution")
		if !ok {
			t.Fatal("prepare_plan_execution missing")
		}
		result, err := definition.ContextHandler(context.Background(), map[string]any{"plan_document": ""}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := invokeEmptyPrepare()
	second := invokeEmptyPrepare()
	if first.StructuredContent["next_actions"] == nil || second.StructuredContent["next_actions"] != nil ||
		!strings.Contains(second.StructuredContent["message"].(string), "stop retrying") {
		t.Fatalf("registry rebuild reset transport guard: first=%#v second=%#v", first, second)
	}
}
