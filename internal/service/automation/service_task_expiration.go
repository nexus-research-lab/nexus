package automation

// 本文件处理任务的显式生命周期截止时间；到期只阻止后续触发，不中断正在执行的 run。

import (
	"context"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) expireTask(ctx context.Context, job automationdomain.CronJob, expiredAt time.Time) error {
	if !job.Enabled {
		return nil
	}
	updated := job
	updated.Enabled = false
	persisted, err := s.repository.UpsertCronJob(ctx, updated)
	if err != nil {
		return err
	}

	s.mu.Lock()
	state := s.jobStates[job.JobID]
	var runtimeUpdate *automationstore.JobRuntimeUpdateInput
	if state != nil {
		state.Job = *persisted
		state.NextRunAt = nil
		snapshot := jobRuntimeUpdateFromState(job.JobID, state)
		runtimeUpdate = &snapshot
	}
	s.mu.Unlock()

	if runtimeUpdate != nil {
		s.persistJobRuntime(ctx, *runtimeUpdate)
	}
	s.recordTaskEvent(ctx, automationdomain.TaskEventActionExpire, *persisted, "", map[string]any{
		"expired_at": expiredAt.UTC(),
		"expires_at": cloneTimePointer(persisted.ExpiresAt),
	})
	return nil
}
