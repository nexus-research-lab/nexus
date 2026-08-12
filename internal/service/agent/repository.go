package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

// Repository 定义 Agent 存储接口。
type Repository interface {
	ListActiveAgents(context.Context, string) ([]protocol.Agent, error)
	ListAgentsByIDs(context.Context, string, []string) ([]protocol.Agent, error)
	GetAgent(context.Context, string, string) (*protocol.Agent, error)
	GetMainAgent(context.Context, string) (*protocol.Agent, error)
	ListAgentContacts(context.Context, string, string) ([]protocol.AgentContact, error)
	GetAgentContact(context.Context, string, string, string) (*protocol.AgentContact, error)
	UpsertAgentContactPair(context.Context, agentrepo.ContactPairRecord) error
	DeleteAgentContactPair(context.Context, string, string) error
	SetAgentContactDirectRoom(context.Context, string, string, string) error
	CreateAgent(context.Context, agentrepo.CreateRecord) (*protocol.Agent, error)
	UpdateAgent(context.Context, agentrepo.UpdateRecord) (*protocol.Agent, error)
	UpdateAgentSkillSelection(context.Context, string, string, string, string) (*protocol.Agent, error)
	UpdateAgentSkillIDsAtVersion(context.Context, string, string, string, int64) (*protocol.Agent, error)
	UpdateAgentDisabledSkillIDsAtVersion(context.Context, string, string, string, int64) (*protocol.Agent, error)
	DeleteAgent(context.Context, string, string) error
	DeleteAgentAtVersion(context.Context, string, string, int64) error
}
