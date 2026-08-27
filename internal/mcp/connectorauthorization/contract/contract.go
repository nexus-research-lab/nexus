// INPUT: server 固化的 owner、主智能体 DM、human principal 与 runtime lease。
// OUTPUT: 不可由模型覆盖的 AuthorizationActor 和三 action 窄服务契约。
// POS: nexus MCP Connector 授权工具到业务服务的可信身份边界。
package contract

import (
	"context"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

// ServerContext 全部来自已验证 transport/runtime 记录。
type ServerContext struct {
	OwnerUserID            string
	CurrentAgentID         string
	BusinessSessionKey     string
	RootRoundID            string
	RuntimeLeaseSessionKey string
	RuntimeLeaseRoundID    string
	PrincipalUserID        string
	PrincipalRole          string
	AuthMethod             string
	AuthSessionID          string
	ContextKind            string
	IsMainAgent            bool
}

// Actor 转换为每次 service 调用仍会重新验证的业务身份。
func (s ServerContext) Actor() connectorsvc.AuthorizationActor {
	return connectorsvc.AuthorizationActor{
		OwnerUserID:            s.OwnerUserID,
		AgentID:                s.CurrentAgentID,
		BusinessSessionKey:     s.BusinessSessionKey,
		RootRoundID:            s.RootRoundID,
		RuntimeLeaseSessionKey: s.RuntimeLeaseSessionKey,
		RuntimeLeaseRoundID:    s.RuntimeLeaseRoundID,
		PrincipalUserID:        s.PrincipalUserID,
		PrincipalRole:          s.PrincipalRole,
		AuthMethod:             s.AuthMethod,
		AuthSessionID:          s.AuthSessionID,
	}
}

// Service 是 MCP action 工具唯一可调用的授权业务面。
type Service interface {
	Start(
		context.Context,
		connectorsvc.AuthorizationActor,
		connectorsvc.AuthorizationStartRequest,
	) (*connectorsvc.AuthorizationFlowView, error)
	Status(
		context.Context,
		connectorsvc.AuthorizationActor,
		connectorsvc.AuthorizationFlowRef,
	) (*connectorsvc.AuthorizationFlowView, error)
	Cancel(
		context.Context,
		connectorsvc.AuthorizationActor,
		connectorsvc.AuthorizationFlowRef,
	) (*connectorsvc.AuthorizationFlowView, error)
}
