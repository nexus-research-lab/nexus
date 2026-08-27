// INPUT: fresh main/ordinary Agent records、认证 principal 与 runtime lease。
// OUTPUT: Channel authorization MCP 只在 owner-main 私有 DM 注入并固化完整 Actor。
// POS: Channel 真人授权应用装配边界测试。
package runtime

import (
	"context"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	channelauthorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
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
