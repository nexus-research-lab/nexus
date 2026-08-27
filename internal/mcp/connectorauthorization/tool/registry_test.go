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

func TestBuildAllOnlyExposesOwnerMainPrivateDM(t *testing.T) {
	base := contract.ServerContext{
		OwnerUserID: "owner-a", CurrentAgentID: "nexus",
		ContextKind: "agent", IsMainAgent: true,
	}
	service := &connectorAuthorizationToolTestService{}
	tools := BuildAll(service, base)
	if len(tools) != 1 {
		t.Fatalf("owner-main DM tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != connectorsvc.ConnectorAuthorizationToolName ||
		tools[0].Annotations == nil ||
		!tools[0].Annotations.Destructive ||
		!tools[0].Annotations.OpenWorld {
		t.Fatalf("authorization tool missing human/open-world boundary: %+v", tools[0])
	}

	ordinary := base
	ordinary.IsMainAgent = false
	if got := BuildAll(
		service, ordinary,
	); len(got) != 0 {
		t.Fatalf("ordinary Agent received Connector auth tools: %+v", got)
	}
	room := base
	room.ContextKind = "room"
	if got := BuildAll(
		service, room,
	); len(got) != 0 {
		t.Fatalf("Room received Connector auth tools: %+v", got)
	}
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

func TestSchemaRejectsIdentityAndProviderSecrets(t *testing.T) {
	schema := authorizationSchema()
	if schema["additionalProperties"] != false {
		t.Fatal("authorization schema must reject extra identity/secret fields")
	}
	properties, _ := schema["properties"].(map[string]any)
	for _, forbidden := range []string{
		"owner_user_id", "agent_id", "session_key", "round_id",
		"state", "code_verifier", "device_code", "auth_code", "token",
	} {
		if _, exists := properties[forbidden]; exists {
			t.Fatalf("authorization schema exposes forbidden field %q", forbidden)
		}
	}
	if _, ok := properties["action"]; !ok {
		t.Fatal("authorization schema missing action")
	}
}
