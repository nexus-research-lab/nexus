// =====================================================
// @File   ：repository.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package agent

import (
	"context"
	"github.com/nexus-research-lab/nexus-core/internal/agentdomain"
)

// Repository 定义 Agent 存储接口。
type Repository interface {
	ListActiveAgents(context.Context) ([]Agent, error)
	GetAgent(context.Context, string) (*Agent, error)
	CreateAgent(context.Context, CreateRecord) (*Agent, error)
	UpdateAgent(context.Context, agentdomain.UpdateRecord) (*Agent, error)
	ArchiveAgent(context.Context, string) error
	ExistsActiveAgentName(context.Context, string, string) (bool, error)
}

type CreateRecord = agentdomain.CreateRecord
type UpdateRecord = agentdomain.UpdateRecord
