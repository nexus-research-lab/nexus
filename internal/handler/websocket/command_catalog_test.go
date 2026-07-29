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

	data := projectCommandCatalog(snapshot, "agent-a")
	if data.Status != protocol.CommandCatalogStatusReady ||
		data.RuntimeKind != "nxs" ||
		data.AgentID != "agent-a" ||
		!strings.HasPrefix(data.Revision, "commands-") ||
		len(data.Commands) != 3 {
		t.Fatalf("catalog = %#v, want scoped ready snapshot", data)
	}
	compact := data.Commands[0]
	if compact.Name != "compact" ||
		compact.Execution != protocol.CommandExecutionRuntimePrompt ||
		!compact.Enabled {
		t.Fatalf("compact = %#v, want runtime-authoritative prompt command", compact)
	}
	mcpPrompt := data.Commands[1]
	if mcpPrompt.Name != "github:review (MCP)" ||
		mcpPrompt.Execution != protocol.CommandExecutionRuntimePrompt ||
		!mcpPrompt.Enabled {
		t.Fatalf("mcp prompt = %#v, want CC-compatible runtime prompt", mcpPrompt)
	}
	review := data.Commands[2]
	if review.Name != "review" ||
		review.Description != "Review code" ||
		review.ArgumentHint != "<target>" ||
		review.Execution != protocol.CommandExecutionRuntimePrompt ||
		!review.Enabled {
		t.Fatalf("review = %#v, want enabled runtime prompt", review)
	}
}

func TestProjectCommandCatalogKeepsLoadingSnapshotEmpty(t *testing.T) {
	data := projectCommandCatalog(runtimectx.CommandCatalogSnapshot{
		Status: runtimectx.CommandCatalogStatusLoading,
		Commands: []agentclient.SlashCommand{{
			Name: "stale",
		}},
	}, "agent-a")

	if data.Status != protocol.CommandCatalogStatusLoading ||
		data.Revision != "" ||
		len(data.Commands) != 0 {
		t.Fatalf("catalog = %#v, want empty loading snapshot", data)
	}
}
