package server

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type stubAutomationAgentResolver struct {
	record    *protocol.Agent
	err       error
	requested string
}

func (s *stubAutomationAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	s.requested = strings.TrimSpace(agentID)
	return s.record, s.err
}

type automationBoundaryService struct {
	automationmcpcontract.Service
	heartbeatAgentIDs []string
	heartbeatOwners   []string
}

func (s *automationBoundaryService) GetHeartbeatStatus(
	ctx context.Context,
	agentID string,
) (*automationdomain.HeartbeatStatus, error) {
	s.heartbeatAgentIDs = append(s.heartbeatAgentIDs, strings.TrimSpace(agentID))
	ownerUserID, _ := authctx.CurrentUserID(ctx)
	s.heartbeatOwners = append(s.heartbeatOwners, ownerUserID)
	return &automationdomain.HeartbeatStatus{
		AgentID:              strings.TrimSpace(agentID),
		EverySeconds:         60,
		TargetMode:           automationdomain.HeartbeatTargetNone,
		AckMaxChars:          120,
		ConfigurationVersion: 1,
	}, nil
}

func TestAutomationMCPBuilderUsesFreshAgentAndLimitsMainAuthorityToPrivateDM(t *testing.T) {
	service := &automationBoundaryService{}
	resolver := &stubAutomationAgentResolver{record: &protocol.Agent{
		AgentID: "nexus", Name: "Fresh Main", OwnerUserID: "owner", IsMain: true,
	}}
	builder := newAutomationMCPBuilder(service, resolver, "Asia/Shanghai")
	dmSession := protocol.BuildAgentSessionKey(
		"nexus",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	ctx := automationAuthenticatedLeaseContext(
		"owner",
		dmSession,
		"round-main",
	)
	servers := builder(
		ctx,
		// runtime snapshot 的 owner/main/name 都故意过期；builder 必须只采用 fresh record。
		&protocol.Agent{
			AgentID: "nexus", Name: "Stale", OwnerUserID: "spoofed", IsMain: false,
		},
		dmSession,
		"round-main",
		"agent",
		"nexus",
		"主会话",
		nil,
		sdkpermission.ModeDefault,
	)
	config := requireAutomationSDKServer(t, servers)
	names := automationToolNames(t, config)
	for _, name := range []string{"create_scheduled_task", "update_scheduled_task", "update_heartbeat"} {
		if !slices.Contains(names, name) {
			t.Fatalf("trusted main DM tools = %v, missing %q", names, name)
		}
	}
	if isError := callAutomationGetHeartbeat(t, config, "worker"); isError {
		t.Fatal("fresh main Agent in its private DM must retain owner-scoped cross-Agent authority")
	}
	if resolver.requested != "nexus" ||
		!slices.Equal(service.heartbeatAgentIDs, []string{"worker"}) ||
		!slices.Equal(service.heartbeatOwners, []string{"owner"}) {
		t.Fatalf(
			"fresh identity was not enforced: requested=%q agents=%v owners=%v",
			resolver.requested,
			service.heartbeatAgentIDs,
			service.heartbeatOwners,
		)
	}
	internalConfig := requireAutomationSDKServer(t, builder(
		runtimectx.WithMCPRoundLease(context.Background(), dmSession, "round-internal"),
		&protocol.Agent{AgentID: "nexus"},
		dmSession,
		"round-internal",
		"agent_internal",
		"nexus",
		"主会话",
		nil,
		sdkpermission.ModeDefault,
	))
	if trusted, internal := automationToolCatalogJSON(t, config), automationToolCatalogJSON(t, internalConfig); trusted != internal {
		t.Fatalf("main DM Automation schema changed across source type\ntrusted=%s\ninternal=%s", trusted, internal)
	}

	roomService := &automationBoundaryService{}
	roomBuilder := newAutomationMCPBuilder(roomService, resolver, "Asia/Shanghai")
	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	roomLease := protocol.BuildRoomAgentSessionKey(
		"conversation-1",
		"nexus",
		protocol.RoomTypeGroup,
	)
	roomConfig := requireAutomationSDKServer(t, roomBuilder(
		automationAuthenticatedLeaseContext("owner", roomLease, "agent-round-main"),
		&protocol.Agent{AgentID: "nexus", IsMain: true},
		roomSession,
		"root-round",
		"room",
		"room-1",
		"测试群",
		nil,
		sdkpermission.ModeDefault,
	))
	if isError := callAutomationGetHeartbeat(t, roomConfig, "worker"); !isError {
		t.Fatal("main Agent inside a Room must not receive owner-wide Automation authority")
	}
	if len(roomService.heartbeatAgentIDs) != 0 {
		t.Fatalf("Room cross-Agent request reached service: %v", roomService.heartbeatAgentIDs)
	}
}

func TestAutomationMCPBuilderOrdinaryAgentStaysSelfScopedInDMAndRoom(t *testing.T) {
	for _, contextKind := range []string{"agent", "room"} {
		t.Run(contextKind, func(t *testing.T) {
			service := &automationBoundaryService{}
			resolver := &stubAutomationAgentResolver{record: &protocol.Agent{
				AgentID: "worker", OwnerUserID: "owner", IsMain: false,
			}}
			builder := newAutomationMCPBuilder(service, resolver, "UTC")
			var sessionKey, roundID, leaseSession, leaseRound, contextID string
			switch contextKind {
			case "agent":
				sessionKey = protocol.BuildAgentSessionKey(
					"worker",
					protocol.SessionChannelWebSocketSegment,
					protocol.RoomTypeDM,
					"main",
					"",
				)
				roundID, leaseSession, leaseRound, contextID = "round-worker", sessionKey, "round-worker", "worker"
			case "room":
				sessionKey = protocol.BuildRoomSharedSessionKey("conversation-2")
				roundID = "root-round"
				leaseSession = protocol.BuildRoomAgentSessionKey(
					"conversation-2",
					"worker",
					protocol.RoomTypeGroup,
				)
				leaseRound, contextID = "agent-round-worker", "room-2"
			}
			config := requireAutomationSDKServer(t, builder(
				automationAuthenticatedLeaseContext("owner", leaseSession, leaseRound),
				// 旧 runtime 快照伪造 IsMain 也不能扩大 fresh ordinary Agent 权限。
				&protocol.Agent{AgentID: "worker", IsMain: true},
				sessionKey,
				roundID,
				contextKind,
				contextID,
				"",
				nil,
				sdkpermission.ModeDefault,
			))
			if !slices.Contains(automationToolNames(t, config), "update_heartbeat") {
				t.Fatal("trusted ordinary Agent must retain self mutation tools")
			}
			if isError := callAutomationGetHeartbeat(t, config, "other"); !isError {
				t.Fatal("ordinary Agent cross-Agent heartbeat read must fail")
			}
			if len(service.heartbeatAgentIDs) != 0 {
				t.Fatalf("ordinary Agent cross-scope request reached service: %v", service.heartbeatAgentIDs)
			}
		})
	}
}

func TestAutomationMCPBuilderKeepsStableSurfaceWithoutGrantingMutation(t *testing.T) {
	service := &automationBoundaryService{}
	record := &protocol.Agent{
		AgentID: "worker", OwnerUserID: "owner", IsMain: false,
	}
	sessionKey := protocol.BuildAgentSessionKey(
		"worker",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"main",
		"",
	)
	validContext := automationAuthenticatedLeaseContext("owner", sessionKey, "round-worker")
	tests := []struct {
		name       string
		resolver   *stubAutomationAgentResolver
		ctx        context.Context
		session    string
		roundID    string
		context    string
		contextID  string
		wantServer bool
	}{
		{
			name: "resolver failure", resolver: &stubAutomationAgentResolver{err: errors.New("stale")},
			ctx: validContext, session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
		},
		{
			name: "resolver record mismatch", resolver: &stubAutomationAgentResolver{record: &protocol.Agent{
				AgentID: "nexus", OwnerUserID: "owner", IsMain: true,
			}},
			ctx: validContext, session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
		},
		{
			name: "missing principal", resolver: &stubAutomationAgentResolver{record: record},
			ctx:     runtimectx.WithMCPRoundLease(context.Background(), sessionKey, "round-worker"),
			session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
			wantServer: true,
		},
		{
			name: "owner mismatch", resolver: &stubAutomationAgentResolver{record: record},
			ctx:     automationAuthenticatedLeaseContext("other-owner", sessionKey, "round-worker"),
			session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
			wantServer: true,
		},
		{
			name: "missing lease", resolver: &stubAutomationAgentResolver{record: record},
			ctx:     automationAuthenticatedContext("owner"),
			session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
			wantServer: true,
		},
		{
			name: "swapped lease", resolver: &stubAutomationAgentResolver{record: record},
			ctx: automationAuthenticatedLeaseContext(
				"owner",
				protocol.BuildAgentSessionKey(
					"worker",
					protocol.SessionChannelWebSocketSegment,
					protocol.RoomTypeDM,
					"other",
					"",
				),
				"round-worker",
			),
			session: sessionKey, roundID: "round-worker", context: "agent", contextID: "worker",
			wantServer: true,
		},
		{
			name: "plain route", resolver: &stubAutomationAgentResolver{record: record},
			ctx:     automationAuthenticatedLeaseContext("owner", "plain", "round-worker"),
			session: "plain", roundID: "round-worker", context: "agent", contextID: "worker",
		},
		{
			name: "source agent mismatch", resolver: &stubAutomationAgentResolver{record: record},
			ctx: validContext, session: sessionKey, roundID: "round-worker", context: "agent", contextID: "other",
			wantServer: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := newAutomationMCPBuilder(service, test.resolver, "UTC")
			servers := builder(
				test.ctx,
				&protocol.Agent{AgentID: "worker", OwnerUserID: "owner", IsMain: true},
				test.session,
				test.roundID,
				test.context,
				test.contextID,
				"",
				nil,
				sdkpermission.ModeDefault,
			)
			if !test.wantServer {
				if len(servers) != 0 {
					t.Fatalf("invalid runtime topology received Automation server: %+v", servers)
				}
				return
			}
			config := requireAutomationSDKServer(t, servers)
			if !slices.Contains(automationToolNames(t, config), "create_scheduled_task") {
				t.Fatalf("stable Session lost Automation mutation schema")
			}
			if !callAutomationTool(t, config, "create_scheduled_task", nil) {
				t.Fatal("untrusted round executed an Automation mutation")
			}
		})
	}
}

func TestAutomationMCPBuilderBackgroundAndExternalSourcesAreReadOnly(t *testing.T) {
	for _, source := range []string{"agent_automation", "agent_external", "room_queue"} {
		t.Run(source, func(t *testing.T) {
			service := &automationBoundaryService{}
			resolver := &stubAutomationAgentResolver{record: &protocol.Agent{
				AgentID: "nexus", OwnerUserID: "owner", IsMain: true,
			}}
			builder := newAutomationMCPBuilder(service, resolver, "UTC")
			config := requireAutomationSDKServer(t, builder(
				context.Background(),
				&protocol.Agent{AgentID: "nexus", OwnerUserID: "spoofed", IsMain: true},
				"untrusted-session",
				"untrusted-round",
				source,
				"untrusted-context",
				"",
				nil,
				sdkpermission.ModeDefault,
			))
			names := automationToolNames(t, config)
			want := []string{
				"find_scheduled_tasks",
				"inspect_scheduled_task",
				"get_scheduled_task_report",
				"get_heartbeat",
			}
			if !slices.Equal(names, want) {
				t.Fatalf("%s tools = %v, want %v", source, names, want)
			}
			if isError := callAutomationGetHeartbeat(t, config, "worker"); !isError {
				t.Fatalf("%s retained main cross-Agent authority", source)
			}
			if isError := callAutomationGetHeartbeat(t, config, "nexus"); isError {
				t.Fatalf("%s must retain current-Agent heartbeat diagnostics", source)
			}
			if !slices.Equal(service.heartbeatOwners, []string{"owner"}) {
				t.Fatalf("%s used stale runtime owner: %v", source, service.heartbeatOwners)
			}
		})
	}
}

func automationAuthenticatedContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     strings.TrimSpace(ownerUserID),
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
	})
}

func automationAuthenticatedLeaseContext(
	ownerUserID string,
	sessionKey string,
	roundID string,
) context.Context {
	return runtimectx.WithMCPRoundLease(
		automationAuthenticatedContext(ownerUserID),
		sessionKey,
		roundID,
	)
}

func requireAutomationSDKServer(
	t *testing.T,
	servers map[string]sdkmcp.ServerConfig,
) sdkmcp.SDKServerConfig {
	t.Helper()
	config, ok := servers[automationmcpcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("missing %s SDK server: %+v", automationmcpcontract.ServerName, servers)
	}
	return config
}

func automationToolNames(t *testing.T, config sdkmcp.SDKServerConfig) []string {
	t.Helper()
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	result, _ := response["result"].(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		names = append(names, strings.TrimSpace(anyStringForAutomationTest(item["name"])))
	}
	return names
}

func automationToolCatalogJSON(t *testing.T, config sdkmcp.SDKServerConfig) string {
	t.Helper()
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	payload, err := json.Marshal(response["result"])
	if err != nil {
		t.Fatalf("marshal tools/list: %v", err)
	}
	return string(payload)
}

func callAutomationGetHeartbeat(
	t *testing.T,
	config sdkmcp.SDKServerConfig,
	agentID string,
) bool {
	t.Helper()
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_heartbeat",
			"arguments": map[string]any{"agent_id": agentID},
		},
	})
	if err != nil {
		t.Fatalf("get_heartbeat: %v", err)
	}
	result, _ := response["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	return isError
}

func callAutomationTool(
	t *testing.T,
	config sdkmcp.SDKServerConfig,
	name string,
	arguments map[string]any,
) bool {
	t.Helper()
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	result, _ := response["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	return isError
}

func anyStringForAutomationTest(value any) string {
	text, _ := value.(string)
	return text
}
