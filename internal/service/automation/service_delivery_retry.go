package automation

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) beginDeliveryRetryBatch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deliveryRetryRunning {
		return false
	}
	s.deliveryRetryRunning = true
	return true
}

func (s *Service) finishDeliveryRetryBatch() {
	s.mu.Lock()
	s.deliveryRetryRunning = false
	s.mu.Unlock()
}

func (s *Service) retryDueDeliveries(ctx context.Context, now time.Time) {
	defer s.finishDeliveryRetryBatch()
	runs, err := s.repository.ListDueDeliveryRetries(ctx, now, maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		s.loggerFor(ctx).Warn("读取待重试投递 run 失败", "err", err)
		return
	}
	for _, run := range runs {
		if err = s.retryDueRunDelivery(ctx, run); err != nil {
			s.loggerFor(ctx).Warn("自动重试投递失败",
				"job_id", run.JobID,
				"run_id", run.RunID,
				"err", err,
			)
		}
	}
}

func (s *Service) retryDueRunDelivery(ctx context.Context, run protocol.CronRun) error {
	job, err := s.repository.GetCronJob(ctx, "", strings.TrimSpace(run.JobID))
	if err != nil {
		return err
	}
	if job == nil {
		message := "scheduled task not found while retrying delivery"
		deadLetterAt := s.nowFn()
		return s.repository.MarkRunDelivery(ctx, automationstore.RunDeliveryUpdateInput{
			RunID:                run.RunID,
			DeliveryStatus:       protocol.DeliveryStatusFailed,
			DeliveryError:        &message,
			DeliveryDeadLetterAt: &deadLetterAt,
		})
	}
	if !job.Enabled {
		deadLetterAt := s.nowFn()
		if err = s.repository.MarkRunDelivery(ctx, automationstore.RunDeliveryUpdateInput{
			RunID:                run.RunID,
			DeliveryStatus:       protocol.DeliveryStatusFailed,
			DeliveryError:        run.DeliveryError,
			DeliveryDeadLetterAt: &deadLetterAt,
		}); err != nil {
			return err
		}
		run.DeliveryStatus = protocol.DeliveryStatusFailed
		run.DeliveryDeadLetterAt = &deadLetterAt
		detail := deliveryRetryTaskEventDetail(run)
		detail["auto_retry_skipped_reason"] = "task_disabled"
		s.recordTaskEvent(ctx, protocol.TaskEventActionAutoRetryDelivery, *job, run.RunID, detail)
		return nil
	}
	updated, err := s.retryRunDelivery(contextForJobOwner(ctx, *job), job.JobID, run.RunID, false)
	if err == nil && updated != nil {
		s.recordTaskEvent(ctx, protocol.TaskEventActionAutoRetryDelivery, *job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	return err
}
