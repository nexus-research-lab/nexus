package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type stubRuntimeAgentResolver struct {
	record *protocol.Agent
}

func (s stubRuntimeAgentResolver) GetAgent(context.Context, string) (*protocol.Agent, error) {
	return s.record, nil
}

func TestConfigurationRuntimeEnvironmentBindsOrdinaryAgentRound(t *testing.T) {
	manager := runtimectx.NewManager()
	service := configurationsvc.NewService(
		config.Config{}, nil, nil, nil, nil, nil, nil, nil, manager,
	)
	agentValue := &protocol.Agent{
		AgentID: "agent-a", OwnerUserID: authctx.SystemUserID,
	}
	cfg := config.Config{Port: 8010, APIPrefix: "/nexus/v1"}
	builder := newConfigurationRuntimeEnvironmentBuilder(
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
