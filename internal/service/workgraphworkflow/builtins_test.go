// INPUT: 内置模板定义、owner 同名历史图与 runtime Slash 调用。
// OUTPUT: 双语拓扑一致、结构合同有效、历史命令优先且内置模板只读的回归证据。
// POS: 内置 WorkGraph catalog 的服务级行为测试。
package workgraphworkflow

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuiltinWorkflowsAreLocalizedValidReadOnlyTemplates(t *testing.T) {
	english := builtinWorkflows("en")
	chinese := builtinWorkflows("zh-CN")
	if len(english) != 4 || len(chinese) != len(english) {
		t.Fatalf("builtin counts = en:%d zh:%d, want 4", len(english), len(chinese))
	}
	for index, workflow := range english {
		localized := chinese[index]
		if !workflow.BuiltIn || !localized.BuiltIn || workflow.ID != localized.ID || workflow.SlashName != localized.SlashName {
			t.Fatalf("builtin identity mismatch: en=%#v zh=%#v", workflow, localized)
		}
		if workflow.Title == localized.Title || workflow.Description == localized.Description {
			t.Fatalf("builtin %s was not localized", workflow.SlashName)
		}
		if workflow.SourceExecutionID != "" || workflow.SourceSessionKey != "" {
			t.Fatalf("builtin %s contains source provenance", workflow.SlashName)
		}
		if len(workflow.Nodes) != len(localized.Nodes) || len(workflow.Dependencies) != len(localized.Dependencies) {
			t.Fatalf("builtin %s localized topology changed", workflow.SlashName)
		}
		for nodeIndex, node := range workflow.Nodes {
			localizedNode := localized.Nodes[nodeIndex]
			if node.LogicalKey != localizedNode.LogicalKey || node.Kind != localizedNode.Kind || node.Role != localizedNode.Role || node.Terminal != localizedNode.Terminal {
				t.Fatalf("builtin %s localized node %d changed structure", workflow.SlashName, nodeIndex)
			}
		}
		preview := protocol.WorkGraphWorkflowPreview{
			PreviewID: "validate-" + workflow.SlashName, SlashName: workflow.SlashName,
			Title: workflow.Title, Description: workflow.Description, Objective: workflow.Objective,
			CompletionCriteria: workflow.CompletionCriteria, Nodes: workflow.Nodes, Dependencies: workflow.Dependencies,
		}
		if _, err := normalizeAndValidateEditorPreview(preview, protocol.ReviseWorkGraphWorkflowPreviewRequest{
			Revision: 1, SlashName: workflow.SlashName, Title: workflow.Title,
			Description: workflow.Description, Objective: workflow.Objective,
			CompletionCriteria: workflow.CompletionCriteria, Nodes: workflow.Nodes, Dependencies: workflow.Dependencies,
		}); err != nil {
			t.Fatalf("builtin %s is not a valid WorkGraph template: %v", workflow.SlashName, err)
		}
	}
}

func TestBuiltinWorkflowCatalogUsesOwnerSavedNameAsUpgradeSafeOverride(t *testing.T) {
	repository := &workflowMemoryRepository{items: map[string]protocol.WorkGraphWorkflow{
		"saved-deep-research": {
			ID: "saved-deep-research", OwnerUserID: "owner-a", SlashName: "deep-research",
			Title: "Existing research command", Description: "Owner saved before upgrade",
			Objective: "preserve-owner-research-semantics",
			Nodes: []protocol.WorkGraphWorkflowNode{{
				LogicalKey: "owner-node", Role: protocol.WorkGraphWorkflowNodeKey,
				Kind: protocol.WorkItemKindProduce, Subject: "Owner node",
				Objective: "Keep the existing command", Deliverable: "Owner result",
				Required: true, Terminal: true,
			}},
		},
	}}
	service := NewService(repository, nil)
	items, err := service.ListLocalized(context.Background(), "owner-a", "zh")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("merged catalog count = %d, want 4", len(items))
	}
	var research protocol.WorkGraphWorkflow
	for _, item := range items {
		if item.SlashName == "deep-research" {
			research = item
		}
	}
	if research.ID != "saved-deep-research" || research.BuiltIn {
		t.Fatalf("deep-research = %#v, want owner saved override", research)
	}
	previewID := "legacy-preview"
	service.previews[previewCacheKey("owner-a", previewID)] = workflowPreviewRecord{
		ownerUserID: "owner-a", savedWorkflowID: research.ID,
		preview: protocol.WorkGraphWorkflowPreview{PreviewID: previewID},
	}
	availability, err := service.CheckSlashNameAvailability(context.Background(), "owner-a", "deep-research", previewID)
	if err != nil || !availability.Available {
		t.Fatalf("legacy saved name availability = %#v, err=%v", availability, err)
	}
	newAvailability, err := service.CheckSlashNameAvailability(context.Background(), "owner-b", "deep-research", "")
	if err != nil || newAvailability.Available {
		t.Fatalf("new builtin name availability = %#v, err=%v", newAvailability, err)
	}
	expanded, err := service.ExpandRuntimePrompt(context.Background(), "owner-a", "/deep-research compare agents")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(expanded, "preserve-owner-research-semantics") || strings.Contains(expanded, "authoritative-evidence") {
		t.Fatalf("runtime expansion ignored owner override: %s", expanded)
	}
}

func TestBuiltinWorkflowCommandsExpandAndCannotBeDeleted(t *testing.T) {
	service := NewService(&workflowMemoryRepository{items: make(map[string]protocol.WorkGraphWorkflow)}, nil)
	commands, err := service.CommandDescriptors(context.Background(), "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 4 || commands[0].Name != "deep-research" {
		t.Fatalf("builtin commands = %#v", commands)
	}
	expanded, err := service.ExpandRuntimePrompt(context.Background(), "owner-a", "/deep-research compare agent runtimes")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"execution-orchestrator", "fresh managed WorkGraph", "compare agent runtimes", "authoritative-evidence-1", "evidence-evaluation-1", "same Execution", "Iteration N+1"} {
		if !strings.Contains(expanded, expected) {
			t.Fatalf("builtin expansion missing %q: %s", expected, expanded)
		}
	}
	deleted, err := service.Delete(context.Background(), "owner-a", builtinWorkflowIDPrefix+"deep-research")
	if err != nil || deleted {
		t.Fatalf("delete builtin = %t, err=%v", deleted, err)
	}
	if _, err = service.PreviewSavedWorkflow(
		context.Background(), "owner-a", builtinWorkflowIDPrefix+"deep-research", "en",
	); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("preview builtin error = %v, want read-only rejection", err)
	}
}

func TestBuiltinWorkflowTopologiesPreserveParallelGatesAndTerminalDeliveries(t *testing.T) {
	tests := []struct {
		slashName    string
		edges        []string
		terminal     string
		adaptiveGate string
		adaptiveText []string
	}{
		{
			slashName: "deep-research",
			edges: []string{
				"frame->research-strategy-1",
				"research-strategy-1->authoritative-evidence-1", "research-strategy-1->contrasting-evidence-1",
				"authoritative-evidence-1->evidence-evaluation-1", "contrasting-evidence-1->evidence-evaluation-1",
				"evidence-evaluation-1->synthesize", "synthesize->verify", "verify->report",
			},
			terminal:     "report",
			adaptiveGate: "evidence-evaluation-1",
			adaptiveText: []string{"same Execution", "Iteration N+1", "latest evaluation"},
		},
		{
			slashName: "build-ship",
			edges: []string{
				"scope->design", "design->implement", "implement->validate", "implement->review",
				"validate->quality-gate-1", "review->quality-gate-1", "quality-gate-1->deliver",
			},
			terminal:     "deliver",
			adaptiveGate: "quality-gate-1",
			adaptiveText: []string{"scope, design, implementation, test, documentation, or external dependency", "parallel revalidation and independent rereview", "latest gate"},
		},
		{
			slashName: "decision-brief",
			edges: []string{
				"frame->evidence", "frame->options", "evidence->evaluate", "options->evaluate",
				"evaluate->challenge", "challenge->recommend",
			},
			terminal:     "recommend",
			adaptiveGate: "challenge",
			adaptiveText: []string{"missing evidence, flawed criteria, missing options", "bounded experiment", "not endless collection"},
		},
		{
			slashName: "review-improve",
			edges: []string{
				"baseline->quality-audit", "baseline->experience-audit",
				"quality-audit->prioritize", "experience-audit->prioritize",
				"prioritize->revise", "revise->verify", "verify->deliver",
			},
			terminal:     "deliver",
			adaptiveGate: "verify",
			adaptiveText: []string{"unresolved findings, regressions, no measurable improvement", "renewed quality or experience audits", "invalid rubric returns to baseline"},
		},
	}

	for _, test := range tests {
		t.Run(test.slashName, func(t *testing.T) {
			workflow := builtinWorkflowBySlashName(test.slashName, "en")
			if workflow == nil {
				t.Fatalf("missing builtin workflow %q", test.slashName)
			}
			gotEdges := make([]string, 0, len(workflow.Dependencies))
			for _, dependency := range workflow.Dependencies {
				gotEdges = append(gotEdges, dependency.DependsOnLogicalKey+"->"+dependency.LogicalKey)
			}
			if strings.Join(gotEdges, ",") != strings.Join(test.edges, ",") {
				t.Fatalf("edges = %v, want %v", gotEdges, test.edges)
			}
			terminalNodes := make([]string, 0, 1)
			for _, node := range workflow.Nodes {
				if !node.Required {
					t.Fatalf("builtin node %q must stay required", node.LogicalKey)
				}
				if node.Terminal {
					terminalNodes = append(terminalNodes, node.LogicalKey)
				}
			}
			if strings.Join(terminalNodes, ",") != test.terminal {
				t.Fatalf("terminal nodes = %v, want [%s]", terminalNodes, test.terminal)
			}
			gate := workflowNodeByLogicalKeyForTest(*workflow, test.adaptiveGate)
			for _, expected := range test.adaptiveText {
				if !strings.Contains(gate.Objective, expected) {
					t.Fatalf("adaptive gate %q objective is missing %q: %s", test.adaptiveGate, expected, gate.Objective)
				}
			}
		})
	}
}

func workflowNodeByLogicalKeyForTest(
	workflow protocol.WorkGraphWorkflow,
	logicalKey string,
) protocol.WorkGraphWorkflowNode {
	for _, node := range workflow.Nodes {
		if node.LogicalKey == logicalKey {
			return node
		}
	}
	return protocol.WorkGraphWorkflowNode{}
}
