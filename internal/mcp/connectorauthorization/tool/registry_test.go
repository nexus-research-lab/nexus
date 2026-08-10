package tool

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectorauthorization/contract"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

type connectorAuthorizationToolTestService struct{}

func (connectorAuthorizationToolTestService) Start(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationStartRequest,
) (*connectorsvc.AuthorizationFlowView, error) {
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Method:   connectorsvc.AuthorizationMethodDevice,
		Status:   connectorsvc.AuthorizationStatusPending,
		UserCode: "SAFE-1234",
	}, nil
}

func (connectorAuthorizationToolTestService) Status(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
	return &connectorsvc.AuthorizationFlowView{
		FlowID: "caf_safe", ConnectorID: "github",
		Status: connectorsvc.AuthorizationStatusPending,
	}, nil
}

func (connectorAuthorizationToolTestService) Cancel(
	context.Context,
	connectorsvc.AuthorizationActor,
	connectorsvc.AuthorizationFlowRef,
) (*connectorsvc.AuthorizationFlowView, error) {
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
	tools := BuildAll(connectorAuthorizationToolTestService{}, base)
	if len(tools) != 3 {
		t.Fatalf("owner-main DM tool count = %d, want 3", len(tools))
	}
	if tools[0].Name != connectorsvc.StartConnectorAuthorizationToolName ||
		tools[0].Annotations == nil ||
		!tools[0].Annotations.Destructive ||
		!tools[0].Annotations.OpenWorld {
		t.Fatalf("start tool missing human/open-world boundary: %+v", tools[0])
	}
	if tools[1].Name != "get_connector_authorization_status" ||
		tools[2].Name != "cancel_connector_authorization" {
		t.Fatalf("unexpected tools: %q %q", tools[1].Name, tools[2].Name)
	}

	ordinary := base
	ordinary.IsMainAgent = false
	if got := BuildAll(
		connectorAuthorizationToolTestService{}, ordinary,
	); len(got) != 0 {
		t.Fatalf("ordinary Agent received Connector auth tools: %+v", got)
	}
	room := base
	room.ContextKind = "room"
	if got := BuildAll(
		connectorAuthorizationToolTestService{}, room,
	); len(got) != 0 {
		t.Fatalf("Room received Connector auth tools: %+v", got)
	}
}

func TestSchemasRejectIdentityAndProviderSecrets(t *testing.T) {
	start := startSchema()
	if start["additionalProperties"] != false {
		t.Fatal("start schema must reject extra identity/secret fields")
	}
	properties, _ := start["properties"].(map[string]any)
	for _, forbidden := range []string{
		"owner_user_id", "agent_id", "session_key", "round_id",
		"state", "code_verifier", "device_code", "auth_code", "token",
	} {
		if _, exists := properties[forbidden]; exists {
			t.Fatalf("start schema exposes forbidden field %q", forbidden)
		}
	}
	ref := flowRefSchema()
	if ref["additionalProperties"] != false {
		t.Fatal("flow ref schema must reject extra fields")
	}
	refProperties, _ := ref["properties"].(map[string]any)
	if len(refProperties) != 2 {
		t.Fatalf("flow ref must contain only flow/connector: %+v", refProperties)
	}
}
