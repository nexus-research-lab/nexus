// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package automation

import (
	"context"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func (s *Service) deliverJobObservation(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	executionSessionKey string,
	observation automationexec.ExecutionObservation,
) jobDeliveryResult {
	return s.deliverJobObservationToTarget(ctx, job, job.Delivery, executionSessionKey, observation)
}
