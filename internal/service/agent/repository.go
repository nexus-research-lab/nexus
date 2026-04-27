package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

// Repository 定义 Agent 存储接口。
type Repository interface {
	ListActiveAgents(context.Context, string) ([]protocol.Agent, error)
	GetAgent(context.Context, string, string) (*protocol.Agent, error)
	GetMainAgent(context.Context, string) (*protocol.Agent, error)
	CreateAgent(context.Context, agentrepo.CreateRecord) (*protocol.Agent, error)
	UpdateAgent(context.Context, agentrepo.UpdateRecord) (*protocol.Agent, error)
	ArchiveAgent(context.Context, string, string) error
	ExistsActiveAgentName(context.Context, string, string, string) (bool, error)
}
