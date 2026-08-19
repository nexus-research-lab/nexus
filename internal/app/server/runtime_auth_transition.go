// INPUT: runtime Manager 的 owner 级关闭能力与认证服务提交回调。
// OUTPUT: 阻断新 runtime admission、撤销 system owner runtime 后再启用认证的原子转场。
// POS: app 装配层连接 auth 与 runtime 生命周期的单向协调边界。
package server

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

type ownerRuntimeCloser interface {
	CloseOwnerSessions(context.Context, string) (int, error)
}

type runtimeAuthTransition struct {
	gate     *runtimeadmission.Gate
	runtimes ownerRuntimeCloser
}

func newRuntimeAuthTransition(runtimes ownerRuntimeCloser) *runtimeAuthTransition {
	return &runtimeAuthTransition{
		gate:     runtimeadmission.NewGate(),
		runtimes: runtimes,
	}
}

func (t *runtimeAuthTransition) BeginRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	return t.gate.Admit(ctx)
}

func (t *runtimeAuthTransition) EnableAuthentication(
	ctx context.Context,
	commit func(context.Context) error,
) error {
	return t.gate.Transition(
		ctx,
		func(revokeContext context.Context) error {
			_, err := t.runtimes.CloseOwnerSessions(revokeContext, authctx.SystemUserID)
			return err
		},
		commit,
	)
}
