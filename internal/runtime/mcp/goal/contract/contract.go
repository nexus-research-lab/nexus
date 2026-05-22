package contract

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const ServerName = "nexus_goal"

// Service 定义 Goal MCP server 需要的最小服务能力。
type Service interface {
	Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error)
	Current(context.Context, string) (*protocol.Goal, error)
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	CompleteByModel(context.Context, string, protocol.CompleteGoalRequest) (*protocol.Goal, error)
	BlockByModel(context.Context, string, protocol.BlockGoalRequest) (*protocol.Goal, error)
}

// ServerContext 绑定当前运行时会话。
type ServerContext struct {
	CurrentSessionKey string
}
