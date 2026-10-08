package workgraphworkflow

import (
	"testing"
)

func TestBuiltinTerminalNodesAreGraphSinks(t *testing.T) {
	for _, workflow := range builtinWorkflows("zh-CN") {
		terminal := make(map[string]bool, len(workflow.Nodes))
		for _, node := range workflow.Nodes {
			terminal[node.LogicalKey] = node.Terminal
		}
		for _, dependency := range workflow.Dependencies {
			if terminal[dependency.DependsOnLogicalKey] {
				t.Fatalf("workflow %q has an edge out of terminal node %q", workflow.SlashName, dependency.DependsOnLogicalKey)
			}
		}
	}
}
