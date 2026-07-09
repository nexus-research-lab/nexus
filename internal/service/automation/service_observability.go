package automation

import (
	"context"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetTaskStatus 返回单个任务的当前状态、健康摘要和最近观测记录。
func (s *Service) GetTaskStatus(ctx context.Context, jobID string, runLimit int, eventLimit int) (*automationdomain.CronTaskStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, automationdomain.ErrJobNotFound
	}
	runs, err := s.ListTaskRuns(ctx, job.JobID)
	if err != nil {
		return nil, err
	}
	events, err := s.ListTaskEvents(ctx, job.JobID, boundedObservabilityLimit(eventLimit, 10, 50))
	if err != nil {
		return nil, err
	}
	runs = limitObservabilityRuns(runs, boundedObservabilityLimit(runLimit, 10, 50))
	return &automationdomain.CronTaskStatus{
		Job:          *job,
		Health:       s.buildCronTaskHealth(*job, runs),
		RecentRuns:   runs,
		RecentEvents: events,
	}, nil
}
