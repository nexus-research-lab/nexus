// INPUT: server 固化的 owner-main DM 业务身份与 live runtime lease。
// OUTPUT: 不可变 service Actor 与四个安全 Channel 授权 action。
// POS: nexus MCP Channel 授权工具的可信 transport 契约。
package contract

import (
	"context"

	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
)

// ServerContext 由 app/runtime builder 注入，模型工具参数不能提供或覆盖任何字段。
type ServerContext struct {
	OwnerUserID       string
	CurrentAgentID    string
	CurrentSessionKey string
	CurrentRoundID    string
	LeaseSessionKey   string
	LeaseRoundID      string
	ContextKind       string
	ContextID         string
	IsMainAgent       bool
	PrincipalRole     string
	AuthMethod        string
	AuthSessionID     string
	LocalSingleUser   bool
}

func (s ServerContext) Actor() authorizationsvc.Actor {
	return authorizationsvc.Actor{
		OwnerUserID:        s.OwnerUserID,
		AgentID:            s.CurrentAgentID,
		SessionKey:         s.CurrentSessionKey,
		RoundID:            s.CurrentRoundID,
		LeaseSessionKey:    s.LeaseSessionKey,
		LeaseRoundID:       s.LeaseRoundID,
		ContextKind:        s.ContextKind,
		ContextID:          s.ContextID,
		IsMainAgent:        s.IsMainAgent,
		PrincipalRole:      s.PrincipalRole,
		AuthMethod:         s.AuthMethod,
		AuthSessionID:      s.AuthSessionID,
		LocalSingleUser:    s.LocalSingleUser,
		RoundLeaseRequired: true,
	}
}

type Service interface {
	Start(context.Context, authorizationsvc.Actor, authorizationsvc.StartInput) (*authorizationsvc.View, error)
	Status(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
	Cancel(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
	RequestVerificationCode(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
}
