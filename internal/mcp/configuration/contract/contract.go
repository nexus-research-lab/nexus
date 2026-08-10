// INPUT: 服务端注入的 owner、Agent、业务 session/root round 与真实 runtime lease。
// OUTPUT: 不可由 MCP 参数覆盖的 configuration Actor 与四工具最小服务契约。
// POS: nexus_config transport 与 configuration 业务服务之间的可信身份边界。
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
	CurrentRoundID    string
	LeaseSessionKey   string
	LeaseRoundID      string
	ContextKind       string
	ContextID         string
	RoomID            string
	ConversationID    string
	SourceContext     string
	IsMainAgent       bool
	PrincipalRole     string
	AuthMethod        string
	AuthSessionID     string
	LocalSingleUser   bool
}

// Actor 把可信 server context 转成配置控制身份。
func (s ServerContext) Actor() configurationsvc.Actor {
	return configurationsvc.Actor{
		OwnerUserID: s.OwnerUserID, AgentID: s.CurrentAgentID,
		SessionKey: s.CurrentSessionKey, RoundID: s.CurrentRoundID,
		LeaseSessionKey: s.LeaseSessionKey, LeaseRoundID: s.LeaseRoundID,
		IsMainAgent: s.IsMainAgent,
		ContextKind: s.ContextKind, ContextID: s.ContextID, RoomID: s.RoomID,
		ConversationID: s.ConversationID,
		SourceContext:  s.SourceContext, PrincipalRole: s.PrincipalRole,
		AuthMethod: s.AuthMethod, AuthSessionID: s.AuthSessionID,
		LocalSingleUser:    s.LocalSingleUser,
		RoundLeaseRequired: true,
	}
}

// Service 是 MCP 所需的最小配置控制面。
type Service interface {
	Inspect(context.Context, configurationsvc.Actor, []string, bool) (*configurationsvc.Inspection, error)
	PlanChange(context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest) (*configurationsvc.ChangePlan, error)
	ApplyChange(context.Context, configurationsvc.Actor, configurationsvc.ChangeRequest) (*configurationsvc.ApplyResult, error)
	ListChanges(context.Context, configurationsvc.Actor, string, int) ([]configurationsvc.AuditRecord, error)
}
