// INPUT: 认证域动态 admission resolver。
// OUTPUT: 绑定安全转场的 runtime admission lease。
// POS: Agent runtime 调用方与认证服务之间的最小依赖倒置边界。
package clientopts

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

var errNilRuntimeAdmissionLease = errors.New("runtime admission returned nil lease")

// AgentRuntimeAdmissionResolver 动态受理 runtime 启动。
//
// 调用方必须把 lease 持有到 runtime session 与 round 均已登记，或启动失败。
type AgentRuntimeAdmissionResolver interface {
	BeginAgentRuntimeAdmission(context.Context) (*runtimeadmission.Lease, error)
}

// BeginAgentRuntimeAdmission 以 fail-closed 方式受理 runtime 启动。
func BeginAgentRuntimeAdmission(
	ctx context.Context,
	resolver AgentRuntimeAdmissionResolver,
) (*runtimeadmission.Lease, error) {
	if resolver == nil {
		return runtimeadmission.NewDetachedLease(ctx), nil
	}
	lease, err := resolver.BeginAgentRuntimeAdmission(ctx)
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		return nil, errors.Join(errors.New("begin Agent runtime admission"), err)
	}
	if lease == nil {
		return nil, errors.Join(
			errors.New("begin Agent runtime admission"),
			errNilRuntimeAdmissionLease,
		)
	}
	return lease, nil
}
