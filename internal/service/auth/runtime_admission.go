// INPUT: 认证存储实时状态与 app 注入的 runtime 安全转场协调器。
// OUTPUT: 带转场 lease 的 runtime admission 与 Linux enforce 隔离要求。
// POS: 认证启用与 runtime 启动共享的动态安全边界。
package auth

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

// RuntimeTransitionCoordinator 由 app 装配层实现 runtime admission 与认证启用转场。
//
// auth 只依赖该中性契约，不反向依赖 runtime Manager。
type RuntimeTransitionCoordinator interface {
	BeginRuntimeAdmission(context.Context) (*runtimeadmission.Lease, error)
	EnableAuthentication(context.Context, func(context.Context) error) error
}

// SetRuntimeTransitionCoordinator 注入认证启用时的 runtime 安全转场协调器。
func (s *Service) SetRuntimeTransitionCoordinator(coordinator RuntimeTransitionCoordinator) {
	s.runtimeTransition = coordinator
}

// BeginAgentRuntimeAdmission 在同一转场 lease 内确认认证状态可读取。
//
// owner 初始化提交前会阻断并撤销所有旧 lease；提交后的新 admission 才能继续启动。
func (s *Service) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	lease := runtimeadmission.NewDetachedLease(ctx)
	var err error
	if s.runtimeTransition != nil {
		lease, err = s.runtimeTransition.BeginRuntimeAdmission(ctx)
		if err != nil {
			return nil, err
		}
	}
	_, err = s.GetState(lease.Context())
	if err != nil {
		lease.Release()
		return nil, err
	}
	return lease, nil
}
