package contract

import (
	"context"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
)

// ServerName 是 MCP server 的注册名。
const ServerName = "nexus_feishu_docx"

// ServerContext 承载当前 owner 作用域。
type ServerContext struct {
	OwnerUserID string
}

// Service 是飞书云文档 MCP server 依赖的服务子集。
type Service interface {
	LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectordomain.ConnectionSnapshot, error)
}
