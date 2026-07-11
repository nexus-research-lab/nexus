package automation

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	return authctx.CurrentUserID(ctx)
}

// ListTasks 列出任务。
func (s *Service) ListTasks(ctx context.Context, agentID string) ([]automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	items, err := s.repository.ListScheduledTasks(ctx, ownerUserID, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]automationdomain.ScheduledTask, 0, len(items))
	for _, item := range items {
		state := s.ensureJobState(item)
		result = append(result, scheduledTaskWithRuntime(item, state))
	}
	return result, nil
}

// CountEnabledTasks 返回启用中的定时任务数量。
func (s *Service) CountEnabledTasks(ctx context.Context, agentID string) (int, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	return s.repository.CountEnabledScheduledTasks(ctx, ownerUserID, strings.TrimSpace(agentID))
}

// GetTask 按 job_id 读取任务。返回 nil 表示未找到。
func (s *Service) GetTask(ctx context.Context, jobID string) (*automationdomain.ScheduledTask, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	state := s.ensureJobState(*job)
	enriched := scheduledTaskWithRuntime(*job, state)
	return &enriched, nil
}
