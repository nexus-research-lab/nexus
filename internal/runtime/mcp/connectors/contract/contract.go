package contract

import (
	"context"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
)

// ServerName 是 MCP server 的注册名。
const ServerName = "nexus_connectors"

// ServerContext 承载当前会话与智能体上下文。
type ServerContext struct {
	OwnerUserID       string
	CurrentAgentID    string
	CurrentSessionKey string
	SourceContextType string
	IsMainAgent       bool
}

// Service 是 connector MCP server 依赖的服务子集。
type Service interface {
	ListActiveConnections(ctx context.Context, ownerUserID string) ([]connectordomain.ConnectionSnapshot, error)
	LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectordomain.ConnectionSnapshot, error)
}
