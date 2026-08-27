package automation

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	if ownerUserID, ok := authctx.CurrentUserID(ctx); ok {
		return ownerUserID, true
	}
	if state, ok := authctx.StateFromContext(ctx); ok && !state.AuthRequired {
		return authctx.SystemUserID, true
	}
	// 未绑定认证状态的后台任务保留空 owner，由专用 scheduler/
	// maintenance 调用显式承担跨 owner 责任。
	return "", false
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
		item = projectTaskPermissionPolicy(item)
		state := s.ensureJobState(item)
		result = append(result, s.scheduledTaskRuntimeSnapshot(item, state))
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
	projected := projectTaskPermissionPolicy(*job)
	job = &projected
	state := s.ensureJobState(*job)
	enriched := s.scheduledTaskRuntimeSnapshot(*job, state)
	return &enriched, nil
}
