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

func TestRuntimeCommandMCPExposesSemanticToolsWithoutCommandEnvelope(t *testing.T) {
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
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := response["result"].(map[string]any)
	tools := result["tools"].([]map[string]any)
	want := map[string]bool{
		protocol.NexusAutomationReadToolName:  false,
		protocol.NexusAutomationPlanToolName:  false,
		protocol.NexusAutomationApplyToolName: false,
	}
	for _, tool := range tools {
		name := tool["name"].(string)
		if name == "command" {
			t.Fatal("retired command envelope is still exposed")
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
		properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for _, retired := range []string{"domain", "action", "input", "request_id"} {
			if _, exists := properties[retired]; exists {
				t.Fatalf("tool %s still exposes retired field %s", name, retired)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("semantic tool %s is missing: %#v", name, tools)
		}
	}
	stagingRoot := filepath.Join(stateRoot, "users", "owner", "runtime", "tmp", "runtime-command-inputs")
	if _, err = os.Stat(stagingRoot); !os.IsNotExist(err) {
		t.Fatalf("runtime command created staging path: %v", err)
	}
}

func TestSemanticRuntimeToolValidatesTheSelectedOperationSchema(t *testing.T) {
	called := false
	operations := []runtimecommand.Operation{
		{
			Name: "text", ReadOnly: true,
			InputSchema: map[string]any{
				"type": "object", "properties": map[string]any{
					"value": map[string]any{"type": "string"},
				}, "required": []string{"value"}, "additionalProperties": false,
			},
		},
		{
			Name: "count", ReadOnly: true,
			InputSchema: map[string]any{
				"type": "object", "properties": map[string]any{
					"value": map[string]any{"type": "integer"},
				}, "required": []string{"value"}, "additionalProperties": false,
			},
			Handler: func(context.Context, map[string]any) (runtimecommand.Result, error) {
				called = true
				return runtimecommand.Result{}, nil
			},
		},
	}
	tool := semanticRuntimeTool(
		"read", "read", operations, true,
		func(ctx context.Context, operationName string, input map[string]any, _ bool, _ string) (any, error) {
			operation, _ := runtimecommand.FindOperation(operations, operationName)
			return operation.Invoke(ctx, input, nil)
		},
	)
	result, err := tool.ContextHandler(context.Background(), map[string]any{
		"operation": "count", "value": 2,
	}, nil)
	if err != nil || result.IsError || !called {
		t.Fatalf("selected operation result = %+v, err = %v, called = %v", result, err, called)
	}
	result, err = tool.ContextHandler(context.Background(), map[string]any{
		"operation": "count", "value": "two",
	}, nil)
	if err != nil || !result.IsError {
		t.Fatalf("invalid selected operation result = %+v, err = %v", result, err)
	}
}
