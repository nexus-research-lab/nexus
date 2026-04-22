// Package connectormcp 提供 nexus_connectors MCP server 入口。
package connectormcp

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	"github.com/nexus-research-lab/nexus/internal/connectors/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/connectors/mcp/tool"
)

// ServerName 是 MCP server 的注册名。
const ServerName = contract.ServerName

// Service 是 connector MCP server 依赖的服务子集。
type Service = contract.Service

// ServerContext 承载当前会话与智能体上下文。
type ServerContext = contract.ServerContext

// NewServer 根据当前上下文构建 nexus_connectors MCP server。
func NewServer(svc Service, sctx ServerContext) *agentclient.SimpleSDKMCPServer {
	return agentclient.NewSimpleSDKMCPServer(ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
