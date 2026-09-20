package workgraphworkflow

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuiltinWorkflowsHaveStableStructureAndArtifactContracts(t *testing.T) {
	workflows := builtinWorkflows("zh-CN")
	if len(workflows) < 13 {
		t.Fatalf("builtin workflow count = %d, want at least 13", len(workflows))
	}
	seen := make(map[string]struct{}, len(workflows))
	for _, workflow := range workflows {
		if workflow.SlashName == "" || workflow.ID == "" {
			t.Fatalf("workflow has empty identity: %#v", workflow)
		}
		if _, ok := seen[workflow.SlashName]; ok {
			t.Fatalf("duplicate builtin slash name %q", workflow.SlashName)
		}
		seen[workflow.SlashName] = struct{}{}
		if !workflow.BuiltIn || workflow.Version != 1 {
			t.Fatalf("workflow %q is not a published builtin: %#v", workflow.SlashName, workflow)
		}
		contract := workflow.ArtifactContract
		if contract == nil || contract.Profile == "" || contract.SourceOfTruth == "" || contract.RenderHint == "" {
			t.Fatalf("workflow %q has incomplete artifact contract: %#v", workflow.SlashName, contract)
		}
		if contract.Primary.Name == "" || contract.Primary.Kind == "" || contract.Primary.Format == "" || contract.Primary.Purpose == "" {
			t.Fatalf("workflow %q has incomplete primary artifact: %#v", workflow.SlashName, contract.Primary)
		}
		keys := make(map[string]protocol.WorkGraphWorkflowNode, len(workflow.Nodes))
		for _, node := range workflow.Nodes {
			if node.LogicalKey == "" || node.Deliverable == "" || len(node.AcceptanceCriteria) == 0 {
				t.Fatalf("workflow %q has incomplete node %q: %#v", workflow.SlashName, node.LogicalKey, node)
			}
			if _, ok := keys[node.LogicalKey]; ok {
				t.Fatalf("workflow %q has duplicate node %q", workflow.SlashName, node.LogicalKey)
			}
			keys[node.LogicalKey] = node
		}
		terminals := 0
		for _, node := range workflow.Nodes {
			if node.Terminal {
				terminals++
			}
		}
		if terminals == 0 {
			t.Fatalf("workflow %q has no terminal node", workflow.SlashName)
		}
		for _, dependency := range workflow.Dependencies {
			if _, ok := keys[dependency.LogicalKey]; !ok {
				t.Fatalf("workflow %q dependency points to missing node %q", workflow.SlashName, dependency.LogicalKey)
			}
			if _, ok := keys[dependency.DependsOnLogicalKey]; !ok {
				t.Fatalf("workflow %q dependency points to missing prerequisite %q", workflow.SlashName, dependency.DependsOnLogicalKey)
			}
		}
	}
}

func TestMethodologyTemplatesExposeCrossValidationAndConvergence(t *testing.T) {
	wanted := map[string]struct{}{
		"first-principles": {}, "mece-strategy": {}, "systems-thinking": {},
		"double-diamond": {}, "jtbd-discovery": {}, "business-model": {},
		"pyramid-brief": {}, "ooda-loop": {}, "ontology-model": {},
	}
	for _, workflow := range builtinWorkflows("en") {
		if _, ok := wanted[workflow.SlashName]; !ok {
			continue
		}
		incoming := make(map[string]int, len(workflow.Nodes))
		for _, dependency := range workflow.Dependencies {
			incoming[dependency.LogicalKey]++
		}
		hasConvergence := false
		for _, count := range incoming {
			if count > 1 {
				hasConvergence = true
				break
			}
		}
		if !hasConvergence {
			t.Fatalf("methodology %q is still a straight line; expected a converging gate", workflow.SlashName)
		}
		hasReview := false
		for _, node := range workflow.Nodes {
			if node.Kind == protocol.WorkItemKindReview || node.Kind == protocol.WorkItemKindVerify {
				hasReview = true
				break
			}
		}
		if !hasReview {
			t.Fatalf("methodology %q has no review or verification stage", workflow.SlashName)
		}
	}
}

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
