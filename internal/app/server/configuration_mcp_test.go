package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	configurationcontract "github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type stubConfigurationAgentResolver struct {
	record *protocol.Agent
}

func (s stubConfigurationAgentResolver) GetAgent(context.Context, string) (*protocol.Agent, error) {
	return s.record, nil
}

type stubConfigurationService struct{}

func (stubConfigurationService) Inspect(
	context.Context, configurationsvc.Actor, []string, bool,
) (*configurationsvc.Inspection, error) {
	return &configurationsvc.Inspection{}, nil
}

func (stubConfigurationService) PlanChange(
	context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest,
) (*configurationsvc.ChangePlan, error) {
	return &configurationsvc.ChangePlan{}, nil
}

func (stubConfigurationService) ApplyChange(
	context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest,
) (*configurationsvc.ApplyResult, error) {
	return &configurationsvc.ApplyResult{}, nil
}

func (stubConfigurationService) ListChanges(
	context.Context, configurationsvc.Actor, string, int,
) ([]configurationsvc.AuditRecord, error) {
	return nil, nil
}

func TestConfigurationMCPBuilderOnlyInjectsMainAgent(t *testing.T) {
	service := stubConfigurationService{}
	mainBuilder := newConfigurationMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "nexus", OwnerUserID: "owner", IsMain: true,
		},
	})
	servers := mainBuilder("nexus", "agent:nexus:dm:main", "", "agent", "nexus", "", nil)
	config, ok := servers[configurationcontract.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("main Agent must receive nexus_config SDK server: %+v", servers)
	}

	workerBuilder := newConfigurationMCPBuilder(service, stubConfigurationAgentResolver{
		record: &protocol.Agent{
			AgentID: "worker", OwnerUserID: "owner", IsMain: false,
		},
	})
	if servers = workerBuilder("worker", "agent:worker:dm:main", "", "agent", "worker", "", nil); len(servers) != 0 {
		t.Fatalf("non-main Agent must not receive global configuration tools: %+v", servers)
	}
}
