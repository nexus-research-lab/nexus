package contract

import (
	"context"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
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
	ListActiveConnections(ctx context.Context, ownerUserID string) ([]connectorsvc.ConnectionSnapshot, error)
	LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectorsvc.ConnectionSnapshot, error)
}
