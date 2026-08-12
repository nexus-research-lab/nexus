package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/config"
	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestResolveGoalMCPSessionKey(t *testing.T) {
	tests := []struct {
		name       string
		sessionKey string
		serverName string
		want       string
	}{
		{
			name:       "shared room goal for group room",
			sessionKey: "agent:devin:ws:group:conversation-1",
			serverName: "room",
			want:       "room:group:conversation-1",
		},
		{
			name:       "keeps room shared key",
			sessionKey: "room:group:conversation-1",
			serverName: "room",
			want:       "room:group:conversation-1",
		},
		{
			name:       "keeps room dm on agent goal",
			sessionKey: "agent:devin:ws:dm:conversation-1",
			serverName: "room",
			want:       "agent:devin:ws:dm:conversation-1",
		},
		{
			name:       "keeps non-room session",
			sessionKey: "agent:devin:ws:group:conversation-1",
			serverName: "automation",
			want:       "agent:devin:ws:group:conversation-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveGoalMCPSessionKey(test.sessionKey, test.serverName); got != test.want {
				t.Fatalf("resolveGoalMCPSessionKey() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAllowsTrustedUserGoalRetargetOnlyForVisibleUserSources(t *testing.T) {
	for source, want := range map[string]bool{
		"agent":           true,
		"room":            true,
		"agent_internal":  false,
		"room_internal":   false,
		"agent_untrusted": false,
		"automation":      false,
	} {
		if got := allowsTrustedUserGoalRetarget(source); got != want {
			t.Fatalf("allowsTrustedUserGoalRetarget(%q) = %t, want %t", source, got, want)
		}
	}
}

type stubGoalMCPAgentResolver struct {
	record  *protocol.Agent
	agentID string
}

func (r *stubGoalMCPAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	r.agentID = agentID
	return r.record, nil
}

type stubGoalMCPService struct {
	createRequest    protocol.CreateGoalRequest
	currentAuthority *protocol.Goal
	authorityErr     error
	authorityCalls   int
	authoritySession string
	authorityOwner   string
	authorityAgent   string
	current          *protocol.Goal
	completeCalls    int
	completeGoalID   string
	completeRequest  protocol.CompleteGoalRequest
}

func (s *stubGoalMCPService) Create(_ context.Context, request protocol.CreateGoalRequest) (*protocol.Goal, error) {
	s.createRequest = request
	return &protocol.Goal{
		ID:         "goal-1",
		SessionKey: request.SessionKey,
		Objective:  request.Objective,
		Status:     protocol.GoalStatusActive,
		Version:    1,
	}, nil
}

func (s *stubGoalMCPService) Current(context.Context, string) (*protocol.Goal, error) {
	return s.current, nil
}

func (*stubGoalMCPService) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (*stubGoalMCPService) RetargetByModel(context.Context, string, protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (*stubGoalMCPService) AuditObjectiveAlignmentByModel(
	context.Context,
	string,
	protocol.AuditGoalObjectiveAlignmentRequest,
) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, nil
}

func (s *stubGoalMCPService) CompleteByModel(
	_ context.Context,
	goalID string,
	request protocol.CompleteGoalRequest,
) (*protocol.Goal, error) {
	s.completeCalls++
	s.completeGoalID = goalID
	s.completeRequest = request
	return &protocol.Goal{
		ID:         goalID,
		SessionKey: s.current.SessionKey,
		Status:     protocol.GoalStatusComplete,
	}, nil
}

func (*stubGoalMCPService) BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (s *stubGoalMCPService) CurrentModelMutationAuthority(
	_ context.Context,
	sessionKey string,
	ownerUserID string,
	agentID string,
) (*protocol.Goal, error) {
	s.authorityCalls++
	s.authoritySession = sessionKey
	s.authorityOwner = ownerUserID
	s.authorityAgent = agentID
	return s.currentAuthority, s.authorityErr
}

func TestGoalMCPBuilderPassesAgentOwnerToCreateGoal(t *testing.T) {
	svc := &stubGoalMCPService{}
	agents := &stubGoalMCPAgentResolver{
		record: &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"},
	}
	builder := newGoalMCPBuilder(config.Config{GoalEnabled: true}, svc)
	authority := runtimectx.NewGoalAuthorityState("", 0, "")
	servers := builder(
		runtimectx.WithGoalAuthorityState(context.Background(), authority),
		agents.record,
		"agent:agent-1:ws:dm:conversation-1",
		" round-1 ",
		"agent",
		"agent-1",
		"Agent",
		authority.ObjectiveRevisionState(),
		sdkpermission.ModeDefault,
	)
	serverConfig, ok := servers[goalmcpcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || serverConfig.Instance == nil {
		t.Fatalf("Goal runtime should inject %s: %+v", goalmcpcontract.ServerName, servers)
	}
	response, err := serverConfig.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "create_goal",
			"arguments": map[string]any{"objective": "Ship durable accounting"},
		},
	})
	if err != nil {
		t.Fatalf("create_goal failed: %v", err)
	}
	if result, _ := response["result"].(map[string]any); result == nil || result["isError"] == true {
		t.Fatalf("create_goal result = %+v, want success", response)
	}
	if svc.createRequest.OwnerUserID != "owner-1" ||
		svc.createRequest.AgentID != "agent-1" ||
		svc.createRequest.RoundID != "round-1" {
		t.Fatalf("create request = %+v, want resolved owner and normalized runtime identity", svc.createRequest)
	}
	createdAuthority, ok := authority.Load()
	if !ok || createdAuthority.GoalID != "goal-1" ||
		createdAuthority.ObjectiveRevision != 1 || createdAuthority.ExecutionID != "" {
		t.Fatalf("authority after create_goal = %#v, ok=%t", createdAuthority, ok)
	}
}

func TestResolveGoalMCPMutationAuthorityBindsDurableOwnerPrivately(t *testing.T) {
	svc := &stubGoalMCPService{currentAuthority: &protocol.Goal{
		ID:         "goal-owned",
		SessionKey: "room:group:conversation-1",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(4),
		},
	}}
	roundAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
	resolved := resolveGoalMCPMutationAuthority(
		context.Background(),
		svc,
		"room:group:conversation-1",
		"room_handoff",
		&protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
		roundAuthority,
	)
	if resolved == roundAuthority {
		t.Fatal("durable owner authority must stay private to nexus_goal")
	}
	authority, ok := resolved.Load()
	if !ok || authority.GoalID != "goal-owned" ||
		authority.ObjectiveRevision != 4 || authority.ExecutionID != "" {
		t.Fatalf("resolved authority = %#v, ok=%t", authority, ok)
	}
	if _, ok = roundAuthority.Load(); ok {
		t.Fatal("durable Goal ownership leaked into the shared Execution authority")
	}
	if svc.authorityCalls != 1 ||
		svc.authoritySession != "room:group:conversation-1" ||
		svc.authorityOwner != "owner-1" || svc.authorityAgent != "agent-lead" {
		t.Fatalf(
			"authority lookup = calls:%d session:%q owner:%q agent:%q",
			svc.authorityCalls,
			svc.authoritySession,
			svc.authorityOwner,
			svc.authorityAgent,
		)
	}
}

func TestResolveGoalMCPMutationAuthorityPreservesHostRoundCapability(t *testing.T) {
	svc := &stubGoalMCPService{currentAuthority: &protocol.Goal{
		ID:     "goal-current",
		Status: protocol.GoalStatusActive,
	}}
	roundAuthority := runtimectx.NewGoalAuthorityState(
		"goal-round",
		2,
		"execution-round",
	)
	resolved := resolveGoalMCPMutationAuthority(
		context.Background(),
		svc,
		"room:group:conversation-1",
		"room",
		&protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
		roundAuthority,
	)
	if resolved != roundAuthority {
		t.Fatal("host-minted round authority was replaced")
	}
	if svc.authorityCalls != 0 {
		t.Fatalf("durable authority lookup calls = %d, want 0", svc.authorityCalls)
	}
}

func TestResolveGoalMCPMutationAuthorityRejectsExternalAndAutomationSources(t *testing.T) {
	for _, test := range []struct {
		name       string
		sessionKey string
		sourceType string
	}{
		{
			name:       "external DM",
			sessionKey: "agent:agent-lead:ws:dm:conversation-1",
			sourceType: "agent_external",
		},
		{
			name:       "Room automation",
			sessionKey: "room:group:conversation-1",
			sourceType: "room_automation",
		},
		{
			name:       "unbound internal Room",
			sessionKey: "room:group:conversation-1",
			sourceType: "room_internal",
		},
		{
			name:       "generic untrusted Room input",
			sessionKey: "room:group:conversation-1",
			sourceType: "room_untrusted",
		},
		{
			name:       "unclaimed Room queue",
			sessionKey: "room:group:conversation-1",
			sourceType: "room_queue",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &stubGoalMCPService{currentAuthority: &protocol.Goal{
				ID:         "goal-owned",
				SessionKey: test.sessionKey,
				Status:     protocol.GoalStatusActive,
			}}
			roundAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
			resolved := resolveGoalMCPMutationAuthority(
				context.Background(),
				svc,
				test.sessionKey,
				test.sourceType,
				&protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
				roundAuthority,
			)
			if resolved != roundAuthority || svc.authorityCalls != 0 {
				t.Fatalf(
					"source %q received durable Goal authority: resolved=%p round=%p calls=%d",
					test.sourceType,
					resolved,
					roundAuthority,
					svc.authorityCalls,
				)
			}
		})
	}
}

func TestGoalMCPBuilderLetsDurableOwnerCompleteInFollowingRound(t *testing.T) {
	current := &protocol.Goal{
		ID:         "goal-owned",
		SessionKey: "room:group:conversation-1",
		Objective:  "finish the Room work",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(4),
		},
	}
	svc := &stubGoalMCPService{
		current:          current,
		currentAuthority: current,
	}
	roundAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
	servers := newGoalMCPBuilder(config.Config{GoalEnabled: true}, svc)(
		runtimectx.WithGoalAuthorityState(context.Background(), roundAuthority),
		&protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
		current.SessionKey,
		"round-follow-up",
		"room",
		"room-1",
		"Room",
		roundAuthority.ObjectiveRevisionState(),
		sdkpermission.ModeDefault,
	)
	serverConfig, ok := servers[goalmcpcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || serverConfig.Instance == nil {
		t.Fatalf("Goal runtime should inject %s: %+v", goalmcpcontract.ServerName, servers)
	}
	response, err := serverConfig.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "update_goal",
			"arguments": map[string]any{"status": "complete"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result, _ := response["result"].(map[string]any); result == nil || result["isError"] == true {
		t.Fatalf("update_goal response = %+v, want success", response)
	}
	if svc.completeCalls != 1 || svc.completeGoalID != current.ID ||
		svc.completeRequest.AgentID != "agent-lead" ||
		svc.completeRequest.RoundID != "round-follow-up" ||
		svc.completeRequest.ExpectedObjectiveRevision != 4 {
		t.Fatalf(
			"complete call = count:%d goal:%q request:%+v",
			svc.completeCalls,
			svc.completeGoalID,
			svc.completeRequest,
		)
	}
	if _, ok = roundAuthority.Load(); ok {
		t.Fatal("owner Goal mutation authority leaked into shared round state")
	}
}
