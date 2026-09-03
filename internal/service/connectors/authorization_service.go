// INPUT: Connector Service、Agent/runtime 身份真相源与人类 principal verifier。
// OUTPUT: 持久化 OAuth/Device 对话授权控制器并注册受控 callback 完成器。
// POS: Connector 对话授权的依赖根；不由 handler 或 MCP 承载业务规则。
package connectors

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

type authorizationAgentService interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type authorizationRoundVerifier interface {
	GetRunningRoundIDs(string) []string
}

type authorizationHumanVerifier interface {
	VerifyInteractiveHuman(context.Context, *authctx.Principal) (*authctx.Principal, error)
}

type authorizationRoleResolver interface {
	ResolveActivePrincipalRole(context.Context, string) (string, error)
}

// AuthorizationControl 实现 owner-main 私有 DM 的持久化授权流程。
type AuthorizationControl struct {
	connectors    *Service
	flows         *connectorstore.AuthorizationFlowStore
	agents        authorizationAgentService
	runtime       authorizationRoundVerifier
	humanVerifier authorizationHumanVerifier
	roleResolver  authorizationRoleResolver
	now           func() time.Time
}

// NewAuthorizationControl 创建并注册受控 OAuth callback/device flow。
func NewAuthorizationControl(
	connectors *Service,
	agents authorizationAgentService,
	runtime authorizationRoundVerifier,
	humanVerifier authorizationHumanVerifier,
	roleResolver authorizationRoleResolver,
) (*AuthorizationControl, error) {
	if connectors == nil || connectors.db == nil {
		return nil, errors.New("Connector authorization control 缺少 Connector service")
	}
	if connectors.credentialKeyringErr != nil {
		return nil, connectors.credentialKeyringErr
	}
	control := &AuthorizationControl{
		connectors: connectors,
		flows: connectorstore.NewAuthorizationFlowStoreWithKeyring(
			connectors.db,
			connectors.driver,
			connectors.credentialKeyring,
			connectors.credentialKeyringErr,
		),
		agents: agents, runtime: runtime,
		humanVerifier: humanVerifier, roleResolver: roleResolver,
		now: func() time.Time { return time.Now().UTC() },
	}
	connectors.authorizationControl = control
	return control, nil
}

func (c *AuthorizationControl) requireReady() error {
	if c == nil || c.connectors == nil || c.flows == nil {
		return errors.New("Connector authorization control 未装配")
	}
	if c.agents == nil || c.runtime == nil {
		return errors.New("Connector authorization control 缺少 Agent/runtime 身份服务")
	}
	if c.humanVerifier == nil || c.roleResolver == nil {
		return errors.New("Connector authorization control 缺少 human principal verifier")
	}
	return nil
}
