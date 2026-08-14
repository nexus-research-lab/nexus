package server

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	managercontract "github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
)

func TestNexusManagerMCPBuilderKeepsSessionSurfaceAndSignsOnlyTrustedRounds(t *testing.T) {
	service := managersvc.NewService(nil, nil, nil, nil, nil)
	ownerContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	mainBuilder := newNexusManagerMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner", IsMain: true,
		},
	})
	mainSession := protocol.BuildAgentSessionKey(
		"nexus", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	mainContext := runtimectx.WithMCPRoundLease(ownerContext, mainSession, "round-main")
	servers := mainBuilder(
		mainContext,
		&protocol.Agent{AgentID: "nexus"},
		mainSession,
		"round-main",
		"agent",
		"nexus",
		"",
		nil,
		sdkpermission.ModeDefault,
	)
	config, ok := servers[managercontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("main Agent must receive nexus_manager SDK server: %+v", servers)
	}

	workerBuilder := newNexusManagerMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "worker", OwnerUserID: "owner",
		},
	})
	workerSession := protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	workerContext := runtimectx.WithMCPRoundLease(ownerContext, workerSession, "round-worker")
	servers = workerBuilder(
		workerContext,
		&protocol.Agent{AgentID: "worker"},
		workerSession,
		"round-worker",
		"agent",
		"worker",
		"",
		nil,
		sdkpermission.ModeDefault,
	)
	if _, ok = servers[managercontract.ServerName]; !ok {
		t.Fatalf("ordinary Agent private DM must receive self workspace manager: %+v", servers)
	}
	workerTools := managerToolNames(t, servers)

	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	roomLeaseSession := protocol.BuildRoomAgentSessionKey(
		"conversation-1", "worker", protocol.RoomTypeGroup,
	)
	roomContext := runtimectx.WithMCPRoundLease(
		ownerContext, roomLeaseSession, "agent-round-room",
	)
	servers = workerBuilder(
		roomContext,
		&protocol.Agent{AgentID: "worker"},
		roomSession,
		"round-root",
		"room",
		"room-1",
		"",
		nil,
		sdkpermission.ModeDefault,
	)
	if _, ok = servers[managercontract.ServerName]; !ok {
		t.Fatalf("trusted Room round must receive current Room read-only manager: %+v", servers)
	}

	rejected := []struct {
		name        string
		ctx         context.Context
		sessionKey  string
		roundID     string
		contextKind string
		contextID   string
		wantServer  bool
	}{
		{
			name: "missing round", ctx: ownerContext, sessionKey: workerSession,
			contextKind: "agent", contextID: "worker",
		},
		{
			name: "mismatched agent", ctx: ownerContext, sessionKey: workerSession,
			roundID: "round", contextKind: "agent", contextID: "other",
			wantServer: true,
		},
		{
			name: "invalid room session", ctx: ownerContext, sessionKey: workerSession,
			roundID: "round", contextKind: "room", contextID: "room-1",
			wantServer: true,
		},
		{
			name: "internal origin", ctx: ownerContext, sessionKey: workerSession,
			roundID: "round", contextKind: "agent_internal", contextID: "worker",
			wantServer: true,
		},
		{
			name: "external origin", ctx: ownerContext, sessionKey: workerSession,
			roundID: "round", contextKind: "agent_external", contextID: "worker",
			wantServer: true,
		},
		{
			name: "missing principal", ctx: context.Background(), sessionKey: workerSession,
			roundID: "round", contextKind: "agent", contextID: "worker",
			wantServer: true,
		},
		{
			name: "mismatched owner",
			ctx: authctx.WithPrincipal(context.Background(), &authctx.Principal{
				UserID: "another-owner", Role: authctx.RoleOwner,
				AuthMethod: authctx.AuthMethodPassword,
			}),
			sessionKey: workerSession, roundID: "round",
			contextKind: "agent", contextID: "worker",
			wantServer: true,
		},
	}
	for _, test := range rejected {
		t.Run(test.name, func(t *testing.T) {
			testContext := test.ctx
			if test.roundID != "" {
				testContext = runtimectx.WithMCPRoundLease(
					testContext, test.sessionKey, "lease-round",
				)
			}
			got := workerBuilder(
				testContext,
				&protocol.Agent{AgentID: "worker"},
				test.sessionKey,
				test.roundID,
				test.contextKind,
				test.contextID,
				"",
				nil,
				sdkpermission.ModeDefault,
			)
			if !test.wantServer {
				if len(got) != 0 {
					t.Fatalf("invalid runtime topology received nexus_manager: %+v", got)
				}
				return
			}
			if tools := managerToolNames(t, got); !slices.Equal(tools, workerTools) {
				t.Fatalf("stable Agent Session tools = %v, want %v", tools, workerTools)
			}
		})
	}
	if got := workerBuilder(
		ownerContext,
		&protocol.Agent{AgentID: "worker"},
		workerSession,
		"round-worker",
		"agent",
		"worker",
		"",
		nil,
		sdkpermission.ModeDefault,
	); len(got) != 0 {
		t.Fatalf("manager without an injected runtime lease must fail closed: %+v", got)
	}
}

func managerToolNames(
	t *testing.T,
	servers map[string]sdkmcp.ServerConfig,
) []string {
	t.Helper()
	config, ok := servers[managercontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("missing %s SDK server: %+v", managercontract.ServerName, servers)
	}
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	result, _ := response["result"].(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		name, _ := item["name"].(string)
		names = append(names, strings.TrimSpace(name))
	}
	return names
}
