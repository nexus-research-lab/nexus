package tool

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type channelAuthorizationToolTestService struct{ lastAction string }

func (s *channelAuthorizationToolTestService) Start(
	context.Context,
	authorizationsvc.Actor,
	authorizationsvc.StartInput,
) (*authorizationsvc.View, error) {
	s.lastAction = actionStart
	return &authorizationsvc.View{}, nil
}

func (s *channelAuthorizationToolTestService) Status(
	context.Context,
	authorizationsvc.Actor,
	string,
) (*authorizationsvc.View, error) {
	s.lastAction = actionStatus
	return &authorizationsvc.View{}, nil
}

func (s *channelAuthorizationToolTestService) Cancel(
	context.Context,
	authorizationsvc.Actor,
	string,
) (*authorizationsvc.View, error) {
	s.lastAction = actionCancel
	return &authorizationsvc.View{}, nil
}

func (s *channelAuthorizationToolTestService) RequestVerificationCode(
	context.Context,
	authorizationsvc.Actor,
	string,
) (*authorizationsvc.View, error) {
	s.lastAction = actionRequestVerificationCode
	return &authorizationsvc.View{}, nil
}

func TestBuildAllOnlyExposesOwnerMainPrivateDMTools(t *testing.T) {
	allowed := contract.ServerContext{
		CurrentAgentID: "main",
		ContextKind:    configurationsvc.ContextKindAgent,
		ContextID:      "main",
		IsMainAgent:    true,
	}
	service := &channelAuthorizationToolTestService{}
	tools := BuildAll(service, allowed)
	if len(tools) != 1 || tools[0].Name != ToolName {
		t.Fatalf("tools = %+v, want one %q tool", tools, ToolName)
	}

	for _, denied := range []contract.ServerContext{
		{CurrentAgentID: "agent", ContextKind: configurationsvc.ContextKindAgent, ContextID: "agent"},
		{CurrentAgentID: "main", ContextKind: configurationsvc.ContextKindRoom, ContextID: "room", IsMainAgent: true},
		{CurrentAgentID: "main", ContextKind: configurationsvc.ContextKindAgent, ContextID: "other", IsMainAgent: true},
	} {
		if tools := BuildAll(service, denied); len(tools) != 0 {
			t.Fatalf("denied context exposed tools: %+v", tools)
		}
	}
}

func TestAuthorizationToolDispatchesActions(t *testing.T) {
	service := &channelAuthorizationToolTestService{}
	item := authorization(service, contract.ServerContext{})
	for _, testCase := range []struct {
		action string
		args   map[string]any
	}{
		{actionStart, map[string]any{"action": actionStart, "channel_type": "telegram"}},
		{actionStatus, map[string]any{"action": actionStatus, "flow_id": "flow-1"}},
		{actionCancel, map[string]any{"action": actionCancel, "flow_id": "flow-1"}},
		{actionRequestVerificationCode, map[string]any{
			"action": actionRequestVerificationCode, "flow_id": "flow-1",
		}},
	} {
		result, err := item.Handler(t.Context(), testCase.args)
		if err != nil || result.IsError || service.lastAction != testCase.action {
			t.Fatalf("action %q dispatch = result=%+v err=%v last=%q", testCase.action, result, err, service.lastAction)
		}
	}
	result, err := item.Handler(t.Context(), map[string]any{"action": "unknown"})
	if err != nil || !result.IsError {
		t.Fatalf("unknown action must return tool error: result=%+v err=%v", result, err)
	}
}
