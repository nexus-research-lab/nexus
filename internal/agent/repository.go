// =====================================================
// @File   ：repository.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package agent

import (
	"context"
)

// Repository 定义 Agent 存储接口。
type Repository interface {
	ListActiveAgents(context.Context, string) ([]Agent, error)
	GetAgent(context.Context, string, string) (*Agent, error)
	GetMainAgent(context.Context, string) (*Agent, error)
	CreateAgent(context.Context, CreateRecord) (*Agent, error)
	UpdateAgent(context.Context, UpdateRecord) (*Agent, error)
	ArchiveAgent(context.Context, string, string) error
	ExistsActiveAgentName(context.Context, string, string, string) (bool, error)
	PromoteMainAgent(context.Context, string, string) (*Agent, error)
}
