package contract

import (
	"context"
	"slices"
	"strings"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
)

// ServerName 是 MCP server 的注册名。
const ServerName = "nexus_connectors"

// ServerContext 承载当前会话与智能体上下文。
type ServerContext struct {
	OwnerUserID         string
	CurrentAgentID      string
	CurrentSessionKey   string
	SourceContextType   string
	SourceContextID     string
	SourceContextLabel  string
	IsMainAgent         bool
	EnabledConnectorIDs []string
}

// ConnectorEnabled 判断当前 Session 是否显式挂载 Connector。
func (s ServerContext) ConnectorEnabled(connectorID string) bool {
	connectorID = strings.TrimSpace(connectorID)
	return connectorID != "" && slices.Contains(s.EnabledConnectorIDs, connectorID)
}

// Service 是 connector MCP server 依赖的服务子集。
type Service interface {
	ListActiveConnections(ctx context.Context, ownerUserID string) ([]connectordomain.ConnectionSnapshot, error)
	LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectordomain.ConnectionSnapshot, error)
}
