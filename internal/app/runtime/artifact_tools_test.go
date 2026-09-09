package runtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type artifactWorkspace struct{ owner, agent string }

func (s *artifactWorkspace) ValidateDeliverables(ctx context.Context, agent string, paths []string) ([]string, error) {
	s.owner, s.agent = authctx.OwnerUserID(ctx), agent
	return paths, nil
}

func TestArtifactToolBuilderBindsTheProducingRoomMember(t *testing.T) {
	service := &artifactWorkspace{}
	builder := NewArtifactToolBuilder(service)
	for _, agentID := range []string{"researcher", "designer"} {
		round := nexusmcp.RoundContext{CommandContext: runtimectx.RuntimeCommandContext{
			Agent:        &protocol.Agent{OwnerUserID: "owner", AgentID: agentID},
			AgentRoundID: agentID + "-round", CoordinatorAgentID: "coordinator",
		}}
		definitions := builder(context.Background(), round)
		if len(definitions) != 1 || definitions[0].Name != "deliver_files" {
			t.Fatal("missing built-in delivery tool")
		}
		result, err := definitions[0].Handler(context.Background(), map[string]any{"paths": []string{"report.pdf"}})
		if err != nil || result.IsError || service.owner != "owner" || service.agent != agentID || result.StructuredContent["agent_round_id"] != agentID+"-round" {
			t.Fatalf("coordinator or ambient identity replaced producer: %+v %+v", service, result)
		}
	}
}
