// INPUT: 到期的 durable delivery retry rows、最新 task 定义与动态投递授权。
// OUTPUT: 有界异步重试批次、dead-letter/next-attempt 状态与下一 deadline 失效通知。
// POS: Automation deadline coordinator 的 delivery worker；同一进程最多运行一个批次。
package automation

import (
	"context"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
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
	s.deliveryDeadlineDirty = true
	s.mu.Unlock()
	s.wakeScheduler()
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

func (s *Service) retryDueRunDelivery(ctx context.Context, run automationdomain.ScheduledTaskRun) error {
	ownerUserID := strings.TrimSpace(run.OwnerUserID)
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(run.JobID))
	if err != nil {
		return err
	}
	if job == nil {
		if strings.TrimSpace(run.DeliveryStatus) == automationdomain.DeliveryStatusPending {
			message := "scheduled task no longer exists; initial delivery was not attempted"
			deadLetterAt := s.nowFn()
			return s.repository.DeadLetterOrphanedPendingRunDelivery(
				ctx,
				run.OwnerUserID,
				run.JobID,
				run.RunID,
				run.DeliveryAttempts,
				&message,
				deadLetterAt,
			)
		}
		message := "scheduled task not found while retrying delivery"
		deadLetterAt := s.nowFn()
		return s.repository.DeadLetterFailedRunDelivery(
			ctx, run.OwnerUserID, run.JobID, run.RunID, run.DeliveryAttempts, &message, deadLetterAt,
		)
	}
	if strings.TrimSpace(run.DeliveryStatus) == automationdomain.DeliveryStatusPending {
		_, deliveryErr := s.deliverPendingRun(contextForJobOwner(ctx, *job), *job, run)
		return deliveryErr
	}
	if !job.Enabled {
		deadLetterAt := s.nowFn()
		if err = s.repository.DeadLetterFailedRunDelivery(
			ctx, run.OwnerUserID, run.JobID, run.RunID, run.DeliveryAttempts, run.DeliveryError, deadLetterAt,
		); err != nil {
			return err
		}
		run.DeliveryStatus = automationdomain.DeliveryStatusFailed
		run.DeliveryDeadLetterAt = &deadLetterAt
		detail := deliveryRetryTaskEventDetail(run)
		detail["auto_retry_skipped_reason"] = "task_disabled"
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionAutoRetryDelivery, *job, run.RunID, detail)
		return nil
	}
	updated, err := s.retryRunDelivery(contextForJobOwner(ctx, *job), job.JobID, run.RunID, false)
	if err == nil && updated != nil {
		s.recordTaskEvent(ctx, automationdomain.TaskEventActionAutoRetryDelivery, *job, run.RunID, deliveryRetryTaskEventDetail(*updated))
	}
	return err
}
