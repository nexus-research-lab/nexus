package automation

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) deadLetterDeletedTaskPendingDeliveries(ctx context.Context, job protocol.CronJob) ([]string, error) {
	runs, err := s.repository.ListRunsByJob(ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID))
	if err != nil {
		return nil, err
	}
	now := s.nowFn()
	message := "scheduled task was deleted before delivery could be retried"
	deadLettered := make([]string, 0)
	for _, run := range runs {
		if !shouldDeadLetterDeletedTaskDelivery(run) {
			continue
		}
		if err = s.repository.MarkRunDelivery(ctx, automationstore.RunDeliveryUpdateInput{
			RunID:                run.RunID,
			DeliveryStatus:       protocol.DeliveryStatusFailed,
			DeliveryError:        &message,
			DeliveryDeadLetterAt: &now,
		}); err != nil {
			return nil, err
		}
		deadLettered = append(deadLettered, strings.TrimSpace(run.RunID))
	}
	return deadLettered, nil
}

func shouldDeadLetterDeletedTaskDelivery(run protocol.CronRun) bool {
	if strings.TrimSpace(run.RunID) == "" || run.DeliveryDeadLetterAt != nil {
		return false
	}
	if strings.TrimSpace(run.Status) == protocol.RunStatusPending ||
		strings.TrimSpace(run.Status) == protocol.RunStatusRunning {
		return false
	}
	return deriveCronRunDeliveryStatus(run) == protocol.DeliveryStatusFailed
}
