// Package contract 定义 nexus_config MCP 的服务与可信 runtime 上下文。
//
// L2 | 父级: internal/mcp/configuration
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package contract

import (
	"context"

	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

// ServerName 是配置 MCP server 注册名。
const ServerName = "nexus_config"

// ServerContext 来自服务端 Agent 记录，不能由工具参数覆盖。
type ServerContext struct {
	OwnerUserID       string
	CurrentAgentID    string
	CurrentSessionKey string
	SourceContext     string
	IsMainAgent       bool
}

// Actor 把可信 server context 转成配置控制身份。
func (s ServerContext) Actor() configurationsvc.Actor {
	return configurationsvc.Actor{
		OwnerUserID: s.OwnerUserID, AgentID: s.CurrentAgentID,
		SessionKey: s.CurrentSessionKey, IsMainAgent: s.IsMainAgent,
		SourceContext: s.SourceContext,
	}
}

// Service 是 MCP 所需的最小配置控制面。
type Service interface {
	Inspect(context.Context, configurationsvc.Actor, []string, bool) (*configurationsvc.Inspection, error)
	PlanChange(context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest) (*configurationsvc.ChangePlan, error)
	ApplyChange(context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest) (*configurationsvc.ApplyResult, error)
	ListChanges(context.Context, configurationsvc.Actor, string, int) ([]configurationsvc.AuditRecord, error)
}
