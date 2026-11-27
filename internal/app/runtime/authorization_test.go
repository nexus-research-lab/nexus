// authorization_test.go 覆盖 app/runtime 渠道、连接器、MCP、执行权威与人工工具审批测试。
package runtime

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	executioncontract "github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	channelauthorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type channelAuthorizationAppService struct {
	lastActor channelauthorizationsvc.Actor
}

func (*channelAuthorizationAppService) Start(
	context.Context,
	channelauthorizationsvc.Actor,
	channelauthorizationsvc.StartInput,
) (*channelauthorizationsvc.View, error) {
	return &channelauthorizationsvc.View{}, nil
}

func (s *channelAuthorizationAppService) Status(
	_ context.Context,
	actor channelauthorizationsvc.Actor,
	flowID string,
) (*channelauthorizationsvc.View, error) {
	s.lastActor = actor
	return &channelauthorizationsvc.View{
		FlowID: flowID,
		Status: "running",
	}, nil
}

func (*channelAuthorizationAppService) Cancel(
	context.Context,
	channelauthorizationsvc.Actor,
	string,
) (*channelauthorizationsvc.View, error) {
	return &channelauthorizationsvc.View{}, nil
}

func (*channelAuthorizationAppService) RequestVerificationCode(
	context.Context,
	channelauthorizationsvc.Actor,
	string,
) (*channelauthorizationsvc.View, error) {
	return &channelauthorizationsvc.View{}, nil
}

func TestChannelAuthorizationMCPBuilderBindsOwnerMainPrivateDM(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"nexus",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	authSessionID := "human-session-a"
	principal := &authctx.Principal{
		UserID:     "owner-a",
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
		SessionID:  &authSessionID,
	}
	ctx := authctx.WithPrincipal(t.Context(), principal)
	ctx = runtimectx.WithRuntimeRoundLease(ctx, sessionKey, "round-a")
	service := &channelAuthorizationAppService{}
	builder := NewChannelAuthorizationToolBuilder(
		service,
		stubRuntimeAgentResolver{record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner-a", IsMain: true,
		}},
	)
	tools := builder(ctx, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
	))
	if len(tools) == 0 {
		t.Fatal("owner-main DM missing Channel authorization tools")
	}
	server := sdktool.NewSimpleSDKMCPServer(nexusMCPServerName, "1.0.0", tools)
	response, err := server.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "channel_authorization",
			"arguments": map[string]any{
				"action": "status", "flow_id": "channel_authorization-safe",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	if result == nil || result["isError"] == true {
		t.Fatalf("status response = %+v", response)
	}
	actor := service.lastActor
	if actor.OwnerUserID != "owner-a" ||
		actor.AgentID != "nexus" ||
		actor.SessionKey != sessionKey ||
		actor.RoundID != "round-a" ||
		actor.LeaseSessionKey != sessionKey ||
		actor.LeaseRoundID != "round-a" ||
		actor.ContextKind != "agent" ||
		actor.ContextID != "nexus" ||
		actor.AuthMethod != authctx.AuthMethodPassword ||
		actor.AuthSessionID != authSessionID ||
		!actor.IsMainAgent ||
		!actor.RoundLeaseRequired {
		t.Fatalf("Channel authorization Actor not fully bound: %+v", actor)
	}

	roomTools := builder(ctx, authorizationMCPRound(
		&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "room", "room-a",
	))
	if len(roomTools) == 0 {
		t.Fatal("same Session lost Channel authorization surface")
	}
	callChannelAuthorizationStatus(
		t,
		sdktool.NewSimpleSDKMCPServer(nexusMCPServerName, "1.0.0", roomTools),
		ctx,
	)
	if service.lastActor.AuthMethod != "" || service.lastActor.AuthSessionID != "" {
		t.Fatalf("non-user round received Channel authorization authority: %+v", service.lastActor)
	}

	for name, denied := range map[string][]sdktool.Tool{
		"ordinary": NewChannelAuthorizationToolBuilder(
			service,
			stubRuntimeAgentResolver{record: &protocol.Agent{
				AgentID: "nexus", OwnerUserID: "owner-a", IsMain: false,
			}},
		)(ctx, authorizationMCPRound(
			&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
		)),
		"missing lease": builder(authctx.WithPrincipal(t.Context(), principal), authorizationMCPRound(
			&protocol.Agent{AgentID: "nexus"}, sessionKey, "round-a", "agent", "nexus",
		)),
	} {
		t.Run(name, func(t *testing.T) {
			if len(denied) != 0 {
				t.Fatalf("unauthorized context received Channel authorization MCP: %+v", denied)
			}
		})
	}
}

func callChannelAuthorizationStatus(
	t *testing.T,
	server *sdktool.SimpleSDKMCPServer,
	ctx context.Context,
) {
	t.Helper()
	response, err := server.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "channel_authorization",
			"arguments": map[string]any{
				"action": "status", "flow_id": "channel_authorization-safe",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	if result == nil || result["isError"] == true {
		t.Fatalf("status response = %+v", response)
	}
}

func authorizationMCPRound(
	agent *protocol.Agent,
	sessionKey string,
	roundID string,
	sourceContextType string,
	sourceContextID string,
) nexusmcp.RoundContext {
	return nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: roundID,
		SourceContextType: sourceContextType, SourceContextID: sourceContextID,
		CommandContext: runtimectx.RuntimeCommandContext{Agent: agent},
	}
}

type connectorMCPServiceStub struct {
	snapshot *connectordomain.ConnectionSnapshot
}

func (stub connectorMCPServiceStub) LoadActiveConnection(
	context.Context,
	string,
	string,
) (*connectordomain.ConnectionSnapshot, error) {
	return stub.snapshot, nil
}

type githubMCPServiceStub struct {
	snapshot         *connectordomain.ConnectionSnapshot
	err              error
	owner, connector string
}

func (s *githubMCPServiceStub) LoadActiveConnection(_ context.Context, owner, connector string) (*connectordomain.ConnectionSnapshot, error) {
	s.owner, s.connector = owner, connector
	return s.snapshot, s.err
}

func TestConnectorBuilderMountsAllBuiltInConnectors(t *testing.T) {
	for _, tc := range []struct{ id, alias, token, transport string }{
		{"github", "github", "oauth-token", "http"},
		{"richmail", "richmail", "paired-token", "http"},
		{"amap", "amap_maps", "map-key", "http"},
		{"didi", "didi_ride", "ride-key", "http"},
		{"dingtalk-ai-table", "dingtalk_ai_table", "https://mcp.dingtalk.com/private-test", "http"},
		{"tencent-docs", "tencent_docs", "docs-token", "http"},
		{"yuque", "yuque", "yuque-token", "stdio"},
		{"feishu-docx", "nexus_feishu_docx", "feishu-token", "sdk"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			ctx := runtimectx.WithEnabledConnectorIDs(context.Background(), []string{tc.id})
			svc := &githubMCPServiceStub{snapshot: &connectordomain.ConnectionSnapshot{ConnectorID: tc.id, AccessToken: tc.token}}
			servers := NewConnectorBuilder(svc)(ctx, &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"}, "", "", "", "", "", nil, sdkpermission.Mode(""))
			if len(servers) != 1 {
				t.Fatalf("mounted server count = %d", len(servers))
			}
			value, ok := servers[tc.alias]
			if !ok {
				t.Fatalf("missing server %q", tc.alias)
			}
			switch tc.transport {
			case "http":
				config, ok := value.(sdkmcp.HTTPServerConfig)
				if !ok || config.URL == "" {
					t.Fatalf("invalid HTTP server: %T", value)
				}
			case "stdio":
				config, ok := value.(sdkmcp.StdioServerConfig)
				if !ok || config.Command == "" || config.Env["YUQUE_PERSONAL_TOKEN"] != tc.token {
					t.Fatal("invalid stdio server")
				}
			case "sdk":
				config, ok := value.(sdkmcp.SDKServerConfig)
				if !ok || config.Instance == nil {
					t.Fatal("invalid SDK server")
				}
			}
			if tc.transport != "sdk" && (svc.owner != "owner-1" || svc.connector != tc.id) {
				t.Fatal("incorrect credential scope")
			}
		})
	}
}

type customConnectorMCPStub struct {
	connectorMCPServiceStub
	owner, id string
}

func (s *customConnectorMCPStub) LoadActiveCustomMCPServer(_ context.Context, owner, id string) (string, map[string]any, error) {
	s.owner, s.id = owner, id
	if id != "custom-mcp:test" {
		return "", nil, nil
	}
	return "my_tools", map[string]any{"type": "http", "url": "https://example.invalid/mcp", "headers": map[string]any{"Authorization": "Bearer custom-token"}}, nil
}
func TestConnectorBuilderMountsSelectedCustomMCP(t *testing.T) {
	svc := &customConnectorMCPStub{}
	builder := NewConnectorBuilder(svc)
	agent := &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"}
	ctx := runtimectx.WithEnabledConnectorIDs(context.Background(), []string{"custom-mcp:test"})
	servers := builder(ctx, agent, "", "", "", "", "", nil, sdkpermission.Mode(""))
	config, ok := servers["my_tools"].(sdkmcp.HTTPServerConfig)
	if !ok || config.Headers["Authorization"] != "Bearer custom-token" {
		t.Fatal("selected custom MCP not mounted with credentials")
	}
	if svc.owner != "owner-1" || svc.id != "custom-mcp:test" {
		t.Fatal("incorrect custom MCP scope")
	}
	servers = builder(context.Background(), agent, "", "", "", "", "", nil, sdkpermission.Mode(""))
	if len(servers) != 0 {
		t.Fatal("unselected custom MCP mounted")
	}
}

func TestRoomCommandActorTrustUsesPhysicalAgentRound(t *testing.T) {
	const (
		ownerID        = "owner-1"
		agentID        = "agent-1"
		conversationID = "conversation-1"
		roomID         = "room-1"
		rootRoundID    = "room-root-round"
		agentRoundID   = "agent-round-1"
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	leaseSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: ownerID, Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	actor := command.Actor{
		OwnerUserID: ownerID, AgentID: agentID,
		SessionKey: sharedSessionKey, RoundID: rootRoundID,
		LeaseSessionKey: leaseSessionKey, LeaseRoundID: agentRoundID,
		SourceContextType: "room", SourceContextID: roomID,
		Round: nexusmcp.RoundContext{CommandContext: runtimectx.RuntimeCommandContext{
			AgentRoundID: agentRoundID,
		}},
	}
	if !trustedCommandActor(ctx, &protocol.Agent{OwnerUserID: ownerID, AgentID: agentID}, actor) {
		t.Fatal("Room actor with an exact physical AgentRoundID was rejected")
	}
	if !actor.SubagentCommandAuthority() {
		t.Fatal("trusted Room actor did not retain Subagent command authority")
	}
}

// Embedding the service interface is enough for contract generation: the test
// never invokes a business mutation, while ResolveCommandContext still sees a
// non-nil complete Execution service.
type executionContractServiceStub struct {
	executioncontract.Service
}

func TestDMGoalContinuationBuildsFullExecutionContract(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-1",
		"",
	)
	goalAuthority := runtimectx.NewGoalAuthorityState("goal-1", 2, "execution-1")
	responsibility := runtimectx.NewResponsibilityAuthorityState(goalAuthority, "execution-1", nil, nil)
	round := nexusRoundForExecutionAuthority(
		sessionKey,
		"round-1",
		&protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		goalAuthority,
		responsibility,
	)
	actor := command.Actor{
		OwnerUserID: "owner-1", AgentID: "agent-1", SessionKey: sessionKey,
		RoundID: "round-1", LeaseSessionKey: sessionKey, LeaseRoundID: "round-1",
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   "agent-1", Round: round,
		GoalMutationAuthority: goalAuthority, GoalResponsibilityState: responsibility,
	}
	if !trustedCommandActor(context.Background(), actor.Round.CommandContext.Agent, actor) {
		t.Fatal("exact DM Goal continuation actor was rejected by runtime admission")
	}

	value, err := HandleExecutionCommand(
		context.Background(),
		&executionContractServiceStub{},
		actor,
		command.Request{Action: command.ActionContract, Operation: "assign_work"},
	)
	if err != nil {
		t.Fatal(err)
	}
	contract, ok := value.(command.Contract)
	if !ok {
		t.Fatalf("contract type = %T, value=%+v", value, value)
	}
	if len(contract.Operations) != 1 || contract.Operations[0].Name != "assign_work" {
		t.Fatalf("assign_work contract = %+v", contract)
	}
	for _, required := range []string{"get_execution", "assign_work", "submit_work", "review_work"} {
		value, err := HandleExecutionCommand(
			context.Background(), &executionContractServiceStub{}, actor,
			command.Request{Action: command.ActionContract, Operation: required},
		)
		if err != nil {
			t.Fatalf("contract %s: %v", required, err)
		}
		operationContract, ok := value.(command.Contract)
		if !ok || len(operationContract.Operations) != 1 || operationContract.Operations[0].Name != required {
			t.Fatalf("contract %s = %#v", required, value)
		}
	}
}

func nexusRoundForExecutionAuthority(
	sessionKey string,
	roundID string,
	agent *protocol.Agent,
	goal *runtimectx.GoalAuthorityState,
	responsibility *runtimectx.ResponsibilityAuthorityState,
) nexusmcp.RoundContext {
	return nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: roundID,
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   agent.AgentID,
		CommandReceipts:   nexusmcp.NewCommandReceiptState(),
		CommandContext: runtimectx.RuntimeCommandContext{
			Agent: agent, ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
			ExecutionID: "execution-1", RootRoundID: roundID,
			SourceContextType: runtimectx.SourceContextGoalContinuation,
			SourceContextID:   agent.AgentID,
			GoalAuthority:     goal, ResponsibilityAuthority: responsibility,
			GoalContinuationAuthority: &runtimectx.GoalContinuationAuthority{
				OwnerUserID: agent.OwnerUserID, AgentID: agent.AgentID, ScopeSessionKey: sessionKey,
				GoalID: "goal-1", ObjectiveRevision: 2, ExecutionID: "execution-1", RootRoundID: roundID,
			},
		},
	}
}

type stubRuntimeAgentResolver struct {
	record *protocol.Agent
}

func (s stubRuntimeAgentResolver) GetAgent(context.Context, string) (*protocol.Agent, error) {
	return s.record, nil
}

func TestConfigurationEnvironmentBindsOrdinaryAgentRound(t *testing.T) {
	manager := runtimectx.NewManager()
	service := configurationsvc.NewService(
		config.Config{}, nil, nil, nil, nil, nil, nil, nil, manager,
	)
	agentValue := &protocol.Agent{
		AgentID: "agent-a", OwnerUserID: authctx.SystemUserID,
	}
	cfg := config.Config{Port: 8010, APIPrefix: "/nexus/v1"}
	builder := NewConfigurationEnvironmentBuilder(
		cfg,
		service,
		stubRuntimeAgentResolver{record: agentValue},
	)
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"dm-a",
		"",
	)
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: false})
	ctx = runtimectx.WithRuntimeRoundLease(ctx, sessionKey, "round-a")
	env, err := builder(ctx, agentValue, sessionKey, "round-a", "agent", agentValue.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if env[protocol.NexusConfigBrokerURLEnvName] != "http://127.0.0.1:8010/nexus/v1/internal/runtime/configuration" ||
		env[protocol.NexusConfigCapabilityTokenEnvName] == "" {
		t.Fatalf("runtime environment = %+v", env)
	}
	if err = manager.StartRound(ctx, sessionKey, "round-a", nil); err != nil {
		t.Fatal(err)
	}
	actor, err := service.ResolveRuntimeCapability(env[protocol.NexusConfigCapabilityTokenEnvName])
	if err != nil {
		t.Fatal(err)
	}
	if actor.AgentID != agentValue.AgentID || actor.ContextID != agentValue.AgentID ||
		actor.IsMainAgent || !actor.RoundLeaseRequired {
		t.Fatalf("runtime actor = %+v", actor)
	}
}
