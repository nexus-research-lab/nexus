// INPUT: 启动时遗留的 scheduled_task deletion jobs。
// OUTPUT: 幂等恢复任务记录、runtime 与隔离 Session 产物清理。
// POS: Automation 删除跨进程恢复入口。
package automation

import (
	"context"
	"errors"

	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
)

// ReconcilePendingDeletions 重放尚未完成的定时任务删除。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) error {
	if s == nil || s.deletion == nil {
		return nil
	}
	jobs, err := s.deletion.ListPending(ctx, deletionsvc.KindScheduledTask)
	if err != nil {
		return err
	}
	errList := make([]error, 0)
	for _, job := range jobs {
		var payload scheduledTaskDeletionPayload
		if decodeErr := deletionsvc.DecodePayload(job, &payload); decodeErr != nil {
			errList = append(errList, s.deletion.Fail(ctx, job, decodeErr))
			continue
		}
		bindScheduledTaskDeletionOwner(&payload, job.OwnerUserID)
		ownerCtx := contextForJobOwner(ctx, payload.Task)
		if _, applyErr := s.applyScheduledTaskDeletion(ownerCtx, job, payload); applyErr != nil {
			errList = append(errList, applyErr)
		}
	}
	return errors.Join(errList...)
}
