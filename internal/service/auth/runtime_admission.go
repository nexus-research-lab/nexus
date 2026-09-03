// INPUT: app 注入的 runtime admission 协调器。
// OUTPUT: Control 状态核对期间持有的 runtime admission lease。
// POS: 身份权威与 runtime 启动之间的最小安全边界。
package auth

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

// RuntimeAdmissionCoordinator 由 app 装配层提供 runtime admission lease。
type RuntimeAdmissionCoordinator interface {
	BeginRuntimeAdmission(context.Context) (*runtimeadmission.Lease, error)
}

func beginAgentRuntimeAdmission(
	ctx context.Context,
	coordinator RuntimeAdmissionCoordinator,
) (*runtimeadmission.Lease, error) {
	lease := runtimeadmission.NewDetachedLease(ctx)
	var err error
	if coordinator != nil {
		lease, err = coordinator.BeginRuntimeAdmission(ctx)
		if err != nil {
			return nil, err
		}
	}
	return lease, nil
}
