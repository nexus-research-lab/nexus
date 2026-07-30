package websocket

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestProjectCommandCatalogSanitizesRuntimeMetadata(t *testing.T) {
	snapshot := runtimectx.CommandCatalogSnapshot{
		Status:      runtimectx.CommandCatalogStatusReady,
		RuntimeKind: agentclient.RuntimeNXS,
		Commands: []agentclient.SlashCommand{
			{
				Name:         "/review",
				Description:  " Review code ",
				ArgumentHint: " <target> ",
				Raw: map[string]any{
					"type":         "prompt",
					"source":       "project",
					"body":         "private command body",
					"path":         "/private/commands/review.md",
					"allowedTools": []string{"Bash"},
				},
			},
			{
				Name:         "compact",
				Description:  "Compact context",
				ArgumentHint: "",
				Raw: map[string]any{
					"type":   "local-jsx",
					"source": "builtin",
				},
			},
			{
				Name:        "github:review (MCP)",
				Description: "Open the GitHub review prompt",
			},
			{Name: "invalid command"},
		},
	}

	data := projectCommandCatalog(snapshot, "agent-a", nil)
	if data.Status != protocol.CommandCatalogStatusReady ||
		data.RuntimeKind != "nxs" ||
		data.AgentID != "agent-a" ||
		!strings.HasPrefix(data.Revision, "commands-") ||
		len(data.Commands) != 3 {
		t.Fatalf("catalog = %#v, want scoped ready snapshot", data)
	}
	compact := data.Commands[0]
	if compact.Name != "compact" ||
		compact.Execution != protocol.CommandExecutionRuntime ||
		!compact.Enabled {
		t.Fatalf("compact = %#v, want runtime-authoritative prompt command", compact)
	}
	mcpPrompt := data.Commands[1]
	if mcpPrompt.Name != "github:review (MCP)" ||
		mcpPrompt.Execution != protocol.CommandExecutionRuntime ||
		!mcpPrompt.Enabled {
		t.Fatalf("mcp prompt = %#v, want CC-compatible runtime prompt", mcpPrompt)
	}
	review := data.Commands[2]
	if review.Name != "review" ||
		review.Description != "Review code" ||
		review.ArgumentHint != "<target>" ||
		review.Execution != protocol.CommandExecutionRuntime ||
		!review.Enabled {
		t.Fatalf("review = %#v, want enabled runtime prompt", review)
	}
}

func TestProjectCommandCatalogKeepsStartingRuntimeCommandsHidden(t *testing.T) {
	data := projectCommandCatalog(runtimectx.CommandCatalogSnapshot{
		Status: runtimectx.CommandCatalogStatusStarting,
		Commands: []agentclient.SlashCommand{{
			Name: "stale",
		}},
	}, "agent-a", []protocol.CommandDescriptor{{
		Name:      "goal",
		Execution: protocol.CommandExecutionHost,
		Enabled:   true,
	}})

	if data.Status != protocol.CommandCatalogStatusStarting ||
		!strings.HasPrefix(data.Revision, "commands-") ||
		len(data.Commands) != 1 ||
		data.Commands[0].Name != "goal" {
		t.Fatalf("catalog = %#v, want host-only starting snapshot", data)
	}
}

func TestProjectCommandCatalogLetsNexusHostCommandWinNameCollision(t *testing.T) {
	data := projectCommandCatalog(runtimectx.CommandCatalogSnapshot{
		Status: runtimectx.CommandCatalogStatusReady,
		Commands: []agentclient.SlashCommand{{
			Name:        "goal",
			Description: "Runtime goal",
		}},
	}, "agent-a", []protocol.CommandDescriptor{{
		Name:        "goal",
		Description: "Nexus goal",
		Execution:   protocol.CommandExecutionHost,
		Enabled:     true,
	}})

	if len(data.Commands) != 1 ||
		data.Commands[0].Execution != protocol.CommandExecutionHost ||
		data.Commands[0].Description != "Nexus goal" {
		t.Fatalf("catalog = %#v, want Nexus host command to reserve /goal", data)
	}
}

func TestProjectCommandCatalogStartupFailureStopsLoading(t *testing.T) {
	failed := projectCommandCatalogStartupFailure(
		runtimectx.CommandCatalogSnapshot{
			Status:     runtimectx.CommandCatalogStatusCold,
			Generation: 3,
		},
	)
	if failed.Status != runtimectx.CommandCatalogStatusUnavailable ||
		failed.Generation != 3 {
		t.Fatalf("failed catalog = %#v, want unavailable generation 3", failed)
	}

	ready := projectCommandCatalogStartupFailure(
		runtimectx.CommandCatalogSnapshot{
			Status:     runtimectx.CommandCatalogStatusReady,
			Generation: 4,
			Commands: []agentclient.SlashCommand{{
				Name: "review",
			}},
		},
	)
	if ready.Status != runtimectx.CommandCatalogStatusReady ||
		ready.Generation != 4 ||
		len(ready.Commands) != 1 {
		t.Fatalf("ready catalog = %#v, want concurrent ready snapshot preserved", ready)
	}
}
