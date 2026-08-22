package websocket

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestProjectCommandCatalogSanitizesRuntimeMetadata(t *testing.T) {
	snapshot := slashcommandsvc.RuntimeCatalogSnapshot{
		Status:      protocol.CommandCatalogStatusReady,
		RuntimeKind: agentclient.RuntimeNXS,
		Commands: []protocol.CommandDescriptor{
			{
				Name:         "/review",
				Description:  " Review code ",
				ArgumentHint: " <target> ",
				Enabled:      true,
			},
			{
				Name:         "compact",
				Description:  "Compact context",
				ArgumentHint: "",
				Enabled:      true,
			},
			{
				Name:        "github:review (MCP)",
				Description: "Open the GitHub review prompt",
				Enabled:     true,
			},
			{Name: "invalid command", Enabled: true},
		},
	}

	data := projectCommandCatalog(snapshot, "agent-a", nil, true)
	if data.Status != protocol.CommandCatalogStatusReady ||
		data.RuntimeKind != "nxs" ||
		data.AgentID != "agent-a" ||
		!strings.HasPrefix(data.Revision, "commands-") ||
		len(data.Commands) != 6 {
		t.Fatalf("catalog = %#v, want scoped ready snapshot", data)
	}
	browser := data.Commands[0]
	if browser.Name != "browser" ||
		browser.Execution != protocol.CommandExecutionRuntime ||
		!browser.Enabled {
		t.Fatalf("browser = %#v, want product runtime prompt", browser)
	}
	compact := data.Commands[1]
	if compact.Name != "compact" ||
		compact.Execution != protocol.CommandExecutionRuntime ||
		!compact.Enabled {
		t.Fatalf("compact = %#v, want runtime-authoritative prompt command", compact)
	}
	mcpPrompt := data.Commands[2]
	if mcpPrompt.Name != "github:review (MCP)" ||
		mcpPrompt.Execution != protocol.CommandExecutionRuntime ||
		!mcpPrompt.Enabled {
		t.Fatalf("mcp prompt = %#v, want CC-compatible runtime prompt", mcpPrompt)
	}
	review := data.Commands[3]
	if review.Name != "review" ||
		review.Description != "Review code" ||
		review.ArgumentHint != "<target>" ||
		review.Execution != protocol.CommandExecutionRuntime ||
		!review.Enabled {
		t.Fatalf("review = %#v, want enabled runtime prompt", review)
	}
	visualize := data.Commands[4]
	if visualize.Name != "visualize" ||
		visualize.Execution != protocol.CommandExecutionRuntime ||
		!visualize.Enabled {
		t.Fatalf("visualize = %#v, want product runtime prompt", visualize)
	}
	workGraph := data.Commands[5]
	if workGraph.Name != "workgraph" ||
		workGraph.Execution != protocol.CommandExecutionRuntime ||
		!workGraph.Enabled {
		t.Fatalf("workgraph = %#v, want fixed collaboration prompt", workGraph)
	}
}

func TestProjectCommandCatalogKeepsUnavailableRuntimeCommandsHidden(t *testing.T) {
	data := projectCommandCatalog(slashcommandsvc.RuntimeCatalogSnapshot{
		Status: protocol.CommandCatalogStatusUnavailable,
		Commands: []protocol.CommandDescriptor{{
			Name:    "stale",
			Enabled: true,
		}},
	}, "agent-a", []protocol.CommandDescriptor{{
		Name:      "goal",
		Execution: protocol.CommandExecutionHost,
		Enabled:   true,
	}}, true)

	if data.Status != protocol.CommandCatalogStatusUnavailable ||
		!strings.HasPrefix(data.Revision, "commands-") ||
		len(data.Commands) != 4 ||
		data.Commands[0].Name != "browser" ||
		data.Commands[1].Name != "goal" ||
		data.Commands[2].Name != "visualize" ||
		data.Commands[3].Name != "workgraph" {
		t.Fatalf("catalog = %#v, want host and product prompts without runtime catalog", data)
	}
}

func TestProjectCommandCatalogKeepsModelOwnedByNexusHost(t *testing.T) {
	data := projectCommandCatalog(slashcommandsvc.RuntimeCatalogSnapshot{
		Status: protocol.CommandCatalogStatusReady,
		Commands: []protocol.CommandDescriptor{{
			Name:        "model",
			Description: "Runtime model",
			Enabled:     true,
		}},
	}, "agent-a", []protocol.CommandDescriptor{{
		Name:        "model",
		Description: "Nexus model",
		Execution:   protocol.CommandExecutionHost,
		Enabled:     true,
	}}, true)

	if len(data.Commands) != 4 ||
		data.Commands[1].Execution != protocol.CommandExecutionHost ||
		data.Commands[1].Description != "Nexus model" {
		t.Fatalf("catalog = %#v, want Nexus host command to reserve /model", data)
	}
}

func TestProjectCommandCatalogIncludesOwnerWorkflowCommands(t *testing.T) {
	data := projectCommandCatalog(
		slashcommandsvc.RuntimeCatalogSnapshot{Status: protocol.CommandCatalogStatusReady},
		"agent-a",
		nil,
		true,
		[]protocol.CommandDescriptor{{
			Name: "deep-research", Description: "Reusable research graph",
			ArgumentHint: "<request>", Execution: protocol.CommandExecutionRuntime, Enabled: true,
		}},
	)
	if len(data.Commands) != 4 || data.Commands[0].Name != "browser" ||
		data.Commands[1].Name != "deep-research" || data.Commands[2].Name != "visualize" ||
		data.Commands[3].Name != "workgraph" {
		t.Fatalf("catalog = %#v, want owner workflow beside fixed product prompts", data)
	}
}

func TestProjectCommandCatalogHidesBrowserWhenServiceUnavailable(t *testing.T) {
	data := projectCommandCatalog(
		slashcommandsvc.RuntimeCatalogSnapshot{Status: protocol.CommandCatalogStatusUnavailable},
		"agent-a",
		nil,
		false,
	)
	if len(data.Commands) != 2 || data.Commands[0].Name != "visualize" || data.Commands[1].Name != "workgraph" {
		t.Fatalf("catalog = %#v, want product prompts without desktop-only Browser", data)
	}
}
