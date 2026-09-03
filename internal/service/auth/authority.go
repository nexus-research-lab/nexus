// INPUT: Desktop 本地主体或外部 Control 的认证能力。
// OUTPUT: HTTP、runtime 与高风险人工确认共用的最小认证权威契约。
// POS: app 装配只依赖此契约，不感知身份来自本地还是独立 Control 进程。
package auth

import (
	"context"
	"net/http"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

// Authority 是 Nexus Server 消费的认证能力；Desktop Local 与 Control adapter 均实现它。
type Authority interface {
	InspectRequest(context.Context, *http.Request) (*Principal, State, error)
	BuildStatusPayload(context.Context, *http.Request) (StatusPayload, error)
	BeginAgentRuntimeAdmission(context.Context) (*runtimeadmission.Lease, error)
	VerifyInteractiveHuman(context.Context, *Principal) (*Principal, error)
	VerifyBoundInteractiveHuman(context.Context, string, string, string) (*Principal, error)
	AcquireBoundInteractiveHumanLease(context.Context, string, string, string) (*Principal, func(), error)
	ResolveActivePrincipalRole(context.Context, string) (string, error)
}
