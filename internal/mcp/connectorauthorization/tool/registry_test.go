package tool

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

type connectorAuthorizationToolTestService struct{ lastAction string }

func (s *connectorAuthorizationToolTestService) Start(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationStartRequest,
) (*connectorsvc.AuthorizationFlowView, error) {
	s.lastAction = actionStart
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Method:   connectorsvc.AuthorizationMethodDevice,
		Status:   connectorsvc.AuthorizationStatusPending,
		UserCode: "SAFE-1234",
	}, nil
}

func (s *connectorAuthorizationToolTestService) Status(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
	s.lastAction = actionStatus
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Status: connectorsvc.AuthorizationStatusPending,
	}, nil
}

func (s *connectorAuthorizationToolTestService) Cancel(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
	s.lastAction = actionCancel
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Status: connectorsvc.AuthorizationStatusCanceled,
	}, nil
}

func TestAuthorizationToolDispatchesActions(t *testing.T) {
	service := &connectorAuthorizationToolTestService{}
	item := authorization(service, contract.ServerContext{})
	for _, testCase := range []struct {
		action string
		args   map[string]any
	}{
		{actionStart, map[string]any{
			"action": actionStart, "request_id": "request-1", "connector_id": "github", "method": "device",
		}},
		{actionStatus, map[string]any{
			"action": actionStatus, "flow_id": "caf_safe", "connector_id": "github",
		}},
		{actionCancel, map[string]any{
			"action": actionCancel, "flow_id": "caf_safe", "connector_id": "github",
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
