package operation

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func TestWorkGraphSchemaRejectsInvalidRevisionAndEmptyNodes(t *testing.T) {
	for _, revision := range []int{0, -1} {
		input := map[string]any{"preview_id": "draft-test", "revision": revision, "selected_revision": 1}
		if err := command.ValidateInput(selectWorkflowPreviewRevisionSchema(), input); err == nil || !strings.Contains(err.Error(), "$.revision") {
			t.Fatalf("revision=%d: %v", revision, err)
		}
	}
	input := map[string]any{"revision": 1, "slash_name": "test", "title": "test", "description": "test", "objective": "test", "nodes": []any{}, "dependencies": []any{}}
	if err := command.ValidateInput(reviseWorkflowPreviewSchema(), input); err == nil || !strings.Contains(err.Error(), "$.nodes") {
		t.Fatalf("empty nodes: %v", err)
	}
}
