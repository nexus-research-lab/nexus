package tool

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/channelauthorization/contract"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestBuildAllOnlyExposesOwnerMainPrivateDMTools(t *testing.T) {
	allowed := contract.ServerContext{
		CurrentAgentID: "main",
		ContextKind:    configurationsvc.ContextKindAgent,
		ContextID:      "main",
		IsMainAgent:    true,
	}
	got := make([]string, 0)
	for _, item := range BuildAll(nil, allowed) {
		got = append(got, item.Name)
	}
	want := []string{
		"start_nexus_channel_authorization",
		"get_nexus_channel_authorization",
		"cancel_nexus_channel_authorization",
		"submit_nexus_channel_authorization_code",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %+v, want %+v", got, want)
	}

	for _, denied := range []contract.ServerContext{
		{CurrentAgentID: "agent", ContextKind: configurationsvc.ContextKindAgent, ContextID: "agent"},
		{CurrentAgentID: "main", ContextKind: configurationsvc.ContextKindRoom, ContextID: "room", IsMainAgent: true},
		{CurrentAgentID: "main", ContextKind: configurationsvc.ContextKindAgent, ContextID: "other", IsMainAgent: true},
	} {
		if tools := BuildAll(nil, denied); len(tools) != 0 {
			t.Fatalf("denied context exposed tools: %+v", tools)
		}
	}
}

func TestVerificationCodeNeverAppearsInAnyToolSchema(t *testing.T) {
	sctx := contract.ServerContext{
		CurrentAgentID: "main",
		ContextKind:    configurationsvc.ContextKindAgent,
		ContextID:      "main",
		IsMainAgent:    true,
	}
	for _, item := range BuildAll(nil, sctx) {
		properties, _ := item.InputSchema["properties"].(map[string]any)
		for _, forbidden := range []string{
			"code", "verify_code", "verification_code",
			"owner_user_id", "agent_id", "session_key", "round_id",
			"lease_session_key", "lease_round_id", "qr_payload",
		} {
			if _, ok := properties[forbidden]; ok {
				t.Fatalf("%s schema exposes forbidden field %s", item.Name, forbidden)
			}
		}
		if additional, ok := item.InputSchema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s must reject additional properties: %+v", item.Name, item.InputSchema)
		}
	}
}
