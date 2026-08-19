// INPUT: fresh main/ordinary Agent records、认证 principal 与 runtime lease。
// OUTPUT: Channel authorization MCP 只在 owner-main 私有 DM 注入并固化完整 Actor。
// POS: Channel 真人授权应用装配边界测试。
package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelauthorizationcontract "github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
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
	builder := newChannelAuthorizationMCPBuilder(
		service,
		stubRuntimeAgentResolver{record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner-a", IsMain: true,
		}},
	)
	servers := builder(
		ctx,
		&protocol.Agent{AgentID: "nexus"},
		sessionKey,
		"round-a",
		"agent",
		"nexus",
		"",
		nil,
		sdkpermission.ModeDefault,
	)
	configValue, ok := servers[channelauthorizationcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || configValue.Instance == nil {
		t.Fatalf("owner-main DM missing Channel authorization MCP: %+v", servers)
	}
	response, err := configValue.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_nexus_channel_authorization",
			"arguments": map[string]any{
				"flow_id": "channel_authorization-safe",
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

	roomServers := builder(
		ctx,
		&protocol.Agent{AgentID: "nexus"},
		sessionKey,
		"round-a",
		"room",
		"room-a",
		"",
		nil,
		sdkpermission.ModeDefault,
	)
	roomConfig, ok := roomServers[channelauthorizationcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || roomConfig.Instance == nil {
		t.Fatalf("same Session lost Channel authorization surface: %+v", roomServers)
	}
	callChannelAuthorizationStatus(t, roomConfig, ctx)
	if service.lastActor.AuthMethod != "" || service.lastActor.AuthSessionID != "" {
		t.Fatalf("non-user round received Channel authorization authority: %+v", service.lastActor)
	}

	for name, denied := range map[string]map[string]sdkmcp.ServerConfig{
		"ordinary": newChannelAuthorizationMCPBuilder(
			service,
			stubRuntimeAgentResolver{record: &protocol.Agent{
				AgentID: "nexus", OwnerUserID: "owner-a", IsMain: false,
			}},
		)(
			ctx,
			&protocol.Agent{AgentID: "nexus"},
			sessionKey,
			"round-a",
			"agent",
			"nexus",
			"",
			nil,
			sdkpermission.ModeDefault,
		),
		"missing lease": builder(
			authctx.WithPrincipal(t.Context(), principal),
			&protocol.Agent{AgentID: "nexus"},
			sessionKey,
			"round-a",
			"agent",
			"nexus",
			"",
			nil,
			sdkpermission.ModeDefault,
		),
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
	config sdkmcp.SDKServerConfig,
	ctx context.Context,
) {
	t.Helper()
	response, err := config.Instance.HandleMessage(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_nexus_channel_authorization",
			"arguments": map[string]any{"flow_id": "channel_authorization-safe"},
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
