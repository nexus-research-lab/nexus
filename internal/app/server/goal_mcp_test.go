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

type stubGoalMCPAgentResolver struct {
	record  *protocol.Agent
	agentID string
}

func (r *stubGoalMCPAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	r.agentID = agentID
	return r.record, nil
}

type stubGoalMCPService struct {
	createRequest protocol.CreateGoalRequest
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

func (*stubGoalMCPService) Current(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
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

func (*stubGoalMCPService) CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	return nil, nil
}

func (*stubGoalMCPService) BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error) {
	return nil, nil
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
