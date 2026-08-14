package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationcontract "github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type stubConfigurationAgentResolver struct {
	record *protocol.Agent
}

func (s stubConfigurationAgentResolver) GetAgent(context.Context, string) (*protocol.Agent, error) {
	return s.record, nil
}

type stubConfigurationService struct {
	lastActor configurationsvc.Actor
}

func (s *stubConfigurationService) Inspect(
	_ context.Context, actor configurationsvc.Actor, _ []string, _ bool,
) (*configurationsvc.Inspection, error) {
	s.lastActor = actor
	return &configurationsvc.Inspection{}, nil
}

func (*stubConfigurationService) PlanChange(
	context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest,
) (*configurationsvc.ChangePlan, error) {
	return &configurationsvc.ChangePlan{}, nil
}

func (*stubConfigurationService) ApplyChange(
	context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest,
) (*configurationsvc.ApplyResult, error) {
	return &configurationsvc.ApplyResult{}, nil
}

func (*stubConfigurationService) ListChanges(
	context.Context, configurationsvc.Actor, string, int,
) ([]configurationsvc.AuditRecord, error) {
	return nil, nil
}

func TestConfigurationMCPBuilderKeepsSessionSurfaceAndSignsOnlyTrustedRounds(t *testing.T) {
	service := &stubConfigurationService{}
	mainSession := protocol.BuildAgentSessionKey(
		"nexus", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	ownerContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	ownerContext = runtimectx.WithMCPRoundLease(
		ownerContext,
		mainSession,
		"round-main",
	)
	mainBuilder := newConfigurationMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner", IsMain: true,
		},
	})
	servers := mainBuilder(
		ownerContext,
		&protocol.Agent{AgentID: "nexus"},
		mainSession, "round-main", "agent", "nexus", "", nil, sdkpermission.ModeDefault,
	)
	config, ok := servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("main Agent must receive nexus_config SDK server: %+v", servers)
	}

	workerBuilder := newConfigurationMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "worker", OwnerUserID: "owner", IsMain: false,
		},
	})
	worker := &protocol.Agent{AgentID: "worker"}
	workerSession := protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	memberContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleMember, AuthMethod: authctx.AuthMethodPassword,
	})
	workerDMContext := runtimectx.WithMCPRoundLease(
		memberContext,
		workerSession,
		"round-worker",
	)
	servers = workerBuilder(
		workerDMContext, worker,
		workerSession, "round-worker", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	if _, ok = servers[configurationcontract.ServerName]; !ok {
		t.Fatalf("ordinary Agent must receive self-scoped configuration tools in its DM: %+v", servers)
	}
	config = servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	callConfigurationInspect(t, config)
	if service.lastActor.SessionKey != workerSession ||
		service.lastActor.RoundID != "round-worker" ||
		service.lastActor.LeaseSessionKey != workerSession ||
		service.lastActor.LeaseRoundID != "round-worker" {
		t.Fatalf("DM actor identity = %+v", service.lastActor)
	}
	queuedContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner,
	})
	queuedContext = authctx.WithQueuedHumanPrincipalBinding(
		queuedContext,
		authctx.QueuedHumanPrincipalBinding{
			UserID: "owner", AuthMethod: authctx.AuthMethodPassword, SessionID: "sess-original",
		},
	)
	queuedContext = runtimectx.WithMCPRoundLease(
		queuedContext,
		workerSession,
		"round-queued",
	)
	servers = workerBuilder(
		queuedContext, worker,
		workerSession, "round-queued", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	config = servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	callConfigurationInspect(t, config)
	if service.lastActor.PrincipalRole != authctx.RoleMember ||
		service.lastActor.AuthMethod != authctx.AuthMethodPassword ||
		service.lastActor.AuthSessionID != "sess-original" {
		t.Fatalf("queued actor trusted synthetic role or lost host auth binding: %+v", service.lastActor)
	}
	crossOwnerQueueContext := authctx.WithQueuedHumanPrincipalBinding(
		queuedContext,
		authctx.QueuedHumanPrincipalBinding{
			UserID: "another-owner", AuthMethod: authctx.AuthMethodPassword, SessionID: "sess-other",
		},
	)
	servers = workerBuilder(
		crossOwnerQueueContext, worker,
		workerSession, "round-queued", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	config = requireConfigurationSDKServer(t, servers)
	callConfigurationInspect(t, config)
	if service.lastActor.ContextID != "" || service.lastActor.AuthMethod != "" {
		t.Fatalf("cross-owner queue received configuration authority: %+v", service.lastActor)
	}
	roomBusinessSession := protocol.BuildRoomSharedSessionKey("conv-a")
	roomLeaseSession := protocol.BuildRoomAgentSessionKey(
		"conv-a", "worker", protocol.RoomTypeGroup,
	)
	roomContext := runtimectx.WithMCPRoundLease(
		memberContext,
		roomLeaseSession,
		"agent-round-worker",
	)
	servers = workerBuilder(
		roomContext, worker,
		roomBusinessSession, "root-round", "room", "room-a", "", nil, sdkpermission.ModeDefault,
	)
	if _, ok = servers[configurationcontract.ServerName]; !ok {
		t.Fatalf("Room Agent must receive dynamically scoped configuration tools: %+v", servers)
	}
	config = servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	callConfigurationInspect(t, config)
	if service.lastActor.SessionKey != roomBusinessSession ||
		service.lastActor.RoundID != "root-round" ||
		service.lastActor.LeaseSessionKey != roomLeaseSession ||
		service.lastActor.LeaseRoundID != "agent-round-worker" ||
		service.lastActor.ConversationID != "conv-a" {
		t.Fatalf("Room actor conflated business identity and runtime lease: %+v", service.lastActor)
	}
	swappedDMLease := runtimectx.WithMCPRoundLease(
		memberContext,
		protocol.BuildAgentSessionKey(
			"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "other", "",
		),
		"round-worker",
	)
	servers = workerBuilder(
		swappedDMLease, worker,
		workerSession, "round-worker", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	config = requireConfigurationSDKServer(t, servers)
	callConfigurationInspect(t, config)
	if service.lastActor.ContextID != "" {
		t.Fatalf("DM business session borrowed another runtime lease: %+v", service.lastActor)
	}
	swappedRoomLease := runtimectx.WithMCPRoundLease(
		memberContext,
		protocol.BuildRoomAgentSessionKey(
			"conv-a", "another-agent", protocol.RoomTypeGroup,
		),
		"agent-round-worker",
	)
	servers = workerBuilder(
		swappedRoomLease, worker,
		roomBusinessSession, "root-round", "room", "room-a", "", nil, sdkpermission.ModeDefault,
	)
	config = requireConfigurationSDKServer(t, servers)
	callConfigurationInspect(t, config)
	if service.lastActor.ContextID != "" || service.lastActor.RoomID != "" {
		t.Fatalf("Room business session borrowed another Agent slot: %+v", service.lastActor)
	}
	for _, missingContext := range []struct {
		contextType string
		contextID   string
	}{
		{contextType: "agent"},
		{contextType: "room"},
		{contextType: "agent", contextID: "another-agent"},
	} {
		if servers = workerBuilder(
			workerDMContext, worker,
			"session", "round", missingContext.contextType, missingContext.contextID, "", nil, sdkpermission.ModeDefault,
		); len(servers) != 0 {
			t.Fatalf(
				"%s:%s must not receive configuration capability: %+v",
				missingContext.contextType,
				missingContext.contextID,
				servers,
			)
		}
	}
	for _, test := range []struct {
		name       string
		ctx        context.Context
		sessionKey string
		roundID    string
		sourceType string
		sourceID   string
	}{
		{name: "agent internal", ctx: workerDMContext, sessionKey: workerSession, roundID: "round-worker", sourceType: "agent_internal", sourceID: "worker"},
		{name: "agent external reply", ctx: workerDMContext, sessionKey: workerSession, roundID: "round-worker", sourceType: "agent_external", sourceID: "worker"},
		{name: "room internal", ctx: roomContext, sessionKey: roomBusinessSession, roundID: "root-round", sourceType: "room_internal", sourceID: "room-a"},
		{name: "unknown round", ctx: workerDMContext, sessionKey: workerSession, roundID: "round-worker", sourceType: "unknown", sourceID: "worker"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stableServers := workerBuilder(
				test.ctx, worker, test.sessionKey, test.roundID,
				test.sourceType, test.sourceID, "", nil, sdkpermission.ModeDefault,
			)
			stableConfig := requireConfigurationSDKServer(t, stableServers)
			callConfigurationInspect(t, stableConfig)
			if service.lastActor.ContextID != "" || service.lastActor.RoomID != "" {
				t.Fatalf("non-user round received configuration authority: %+v", service.lastActor)
			}
		})
	}
	servers = workerBuilder(
		runtimectx.WithMCPRoundLease(context.Background(), workerSession, "round-worker"), worker,
		workerSession, "round-worker", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	config = requireConfigurationSDKServer(t, servers)
	callConfigurationInspect(t, config)
	if service.lastActor.ContextID != "" || service.lastActor.AuthMethod != "" {
		t.Fatalf("missing principal received configuration authority: %+v", service.lastActor)
	}
	mismatched := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "another-owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	mismatched = runtimectx.WithMCPRoundLease(mismatched, workerSession, "round-worker")
	servers = workerBuilder(
		mismatched, worker,
		workerSession, "round-worker", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	)
	config = requireConfigurationSDKServer(t, servers)
	callConfigurationInspect(t, config)
	if service.lastActor.ContextID != "" || service.lastActor.AuthMethod != "" {
		t.Fatalf("mismatched principal received configuration authority: %+v", service.lastActor)
	}
	if servers = workerBuilder(
		memberContext, worker,
		workerSession, "round-worker", "agent", "worker", "", nil, sdkpermission.ModeDefault,
	); len(servers) != 0 {
		t.Fatalf("missing runtime lease must fail closed: %+v", servers)
	}
}

func requireConfigurationSDKServer(
	t *testing.T,
	servers map[string]sdkmcp.ServerConfig,
) sdkmcp.SDKServerConfig {
	t.Helper()
	config, ok := servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("missing %s SDK server: %+v", configurationcontract.ServerName, servers)
	}
	return config
}

func callConfigurationInspect(t *testing.T, config sdkmcp.SDKServerConfig) {
	t.Helper()
	response, err := config.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "inspect_nexus_configuration",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("inspect nexus configuration: %v", err)
	}
	result, _ := response["result"].(map[string]any)
	if result == nil || result["isError"] == true {
		t.Fatalf("inspect nexus configuration response = %+v", response)
	}
}
