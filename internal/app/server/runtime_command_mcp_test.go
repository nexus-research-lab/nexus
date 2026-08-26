package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

func TestRuntimeCommandMCPUsesStructuredInputWithoutStagingFile(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	agent := &protocol.Agent{AgentID: "worker", OwnerUserID: "owner"}
	sessionKey := protocol.BuildAgentSessionKey(
		agent.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	ctx = runtimectx.WithRuntimeRoundLease(ctx, sessionKey, "round-1")
	builder := newRuntimeCommandMCPServerBuilder(
		config.Config{},
		stubRuntimeAgentResolver{record: agent},
		automationsvc.NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil),
		nil,
		nil,
		nil,
	)
	servers, err := builder(ctx, runtimecommand.RoundContext{
		SessionKey:         sessionKey,
		RoundID:            "round-1",
		SourceContextType:  "agent",
		SourceContextID:    agent.AgentID,
		SourceContextLabel: "Worker",
		Receipts:           runtimecommand.NewReceiptState(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: agent, ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
			RootRoundID: "round-1", SourceContextType: "agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	configValue, ok := servers["nexus"].(sdkmcp.SDKServerConfig)
	if !ok || configValue.Instance == nil {
		t.Fatalf("runtime MCP server = %#v", servers)
	}
	response, err := configValue.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": runtimeCommandMCPToolName,
			"arguments": map[string]any{
				"domain": runtimecommand.DomainAutomation,
				"action": runtimecommand.ActionContract,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	if structured["mutation_allowed"] != true {
		t.Fatalf("runtime command result = %#v", result)
	}
	stagingRoot := filepath.Join(stateRoot, "users", "owner", "runtime", "tmp", "runtime-command-inputs")
	if _, err = os.Stat(stagingRoot); !os.IsNotExist(err) {
		t.Fatalf("runtime command created staging path: %v", err)
	}
}
