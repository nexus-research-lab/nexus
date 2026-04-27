package automation

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// ListTasks 列出任务。
func (s *Service) ListTasks(ctx context.Context, agentID string) ([]CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	items, err := s.repository.ListCronJobs(ctx, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]CronJob, 0, len(items))
	for _, item := range items {
		state := s.ensureJobState(item)
		enriched := item
		enriched.Running = state.Running
		enriched.NextRunAt = cloneTimePointer(state.NextRunAt)
		enriched.LastRunAt = cloneTimePointer(state.LastRunAt)
		result = append(result, enriched)
	}
	return result, nil
}

// CountEnabledTasks 返回启用中的定时任务数量。
func (s *Service) CountEnabledTasks(ctx context.Context, agentID string) (int, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	return s.repository.CountEnabledCronJobs(ctx, strings.TrimSpace(agentID))
}

// GetTask 按 job_id 读取任务。返回 nil 表示未找到。
func (s *Service) GetTask(ctx context.Context, jobID string) (*CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.repository.GetCronJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	state := s.ensureJobState(*job)
	enriched := *job
	enriched.Running = state.Running
	enriched.NextRunAt = cloneTimePointer(state.NextRunAt)
	enriched.LastRunAt = cloneTimePointer(state.LastRunAt)
	return &enriched, nil
}

// CreateTask 创建任务。
func (s *Service) CreateTask(ctx context.Context, input CreateJobInput) (*CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	normalized := input.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	if err := s.validateAgentAndTarget(ctx, normalized.AgentID, normalized.SessionTarget); err != nil {
		return nil, err
	}

	job := CronJob{
		JobID:         s.idFactory("cron"),
		Name:          normalized.Name,
		AgentID:       normalized.AgentID,
		Schedule:      normalized.Schedule,
		Instruction:   normalized.Instruction,
		SessionTarget: normalized.SessionTarget,
		Delivery:      normalized.Delivery,
		Source:        normalized.Source,
		Enabled:       normalized.Enabled,
	}
	created, err := s.repository.UpsertCronJob(ctx, job)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*created)
	result := *created
	result.NextRunAt = cloneTimePointer(state.NextRunAt)
	result.LastRunAt = cloneTimePointer(state.LastRunAt)
	result.Running = state.Running
	return &result, nil
}

// UpdateTask 更新任务。
func (s *Service) UpdateTask(ctx context.Context, jobID string, input UpdateJobInput) (*CronJob, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	current, err := s.repository.GetCronJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrJobNotFound
	}

	next := *current
	if input.Name != nil {
		next.Name = strings.TrimSpace(*input.Name)
	}
	if input.Schedule != nil {
		next.Schedule = input.Schedule.Normalized()
	}
	if input.Instruction != nil {
		next.Instruction = strings.TrimSpace(*input.Instruction)
	}
	if input.SessionTarget != nil {
		next.SessionTarget = input.SessionTarget.Normalized()
	}
	if input.Delivery != nil {
		next.Delivery = input.Delivery.Normalized()
	}
	if input.Source != nil {
		next.Source = input.Source.Normalized()
	}
	if input.Enabled != nil {
		next.Enabled = *input.Enabled
	}

	createLike := CreateJobInput{
		Name:          next.Name,
		AgentID:       next.AgentID,
		Schedule:      next.Schedule,
		Instruction:   next.Instruction,
		SessionTarget: next.SessionTarget,
		Delivery:      next.Delivery,
		Source:        next.Source,
		Enabled:       next.Enabled,
	}
	if err = createLike.Validate(); err != nil {
		return nil, err
	}
	if err = s.validateAgentAndTarget(ctx, next.AgentID, next.SessionTarget); err != nil {
		return nil, err
	}

	updated, err := s.repository.UpsertCronJob(ctx, next)
	if err != nil {
		return nil, err
	}
	state := s.ensureJobState(*updated)
	result := *updated
	result.NextRunAt = cloneTimePointer(state.NextRunAt)
	result.LastRunAt = cloneTimePointer(state.LastRunAt)
	result.Running = state.Running
	return &result, nil
}

// UpdateTaskStatus 切换任务启停。
func (s *Service) UpdateTaskStatus(ctx context.Context, jobID string, enabled bool) (*CronJob, error) {
	return s.UpdateTask(ctx, jobID, UpdateJobInput{Enabled: &enabled})
}

// DeleteTask 删除任务。
func (s *Service) DeleteTask(ctx context.Context, jobID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	current, err := s.repository.GetCronJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return err
	}
	if current == nil {
		return ErrJobNotFound
	}
	if err = s.repository.DeleteCronJob(ctx, current.JobID); err != nil {
		return err
	}
	if err = s.cleanupIsolatedAutomationSessions(ctx, *current); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.jobStates, current.JobID)
	s.mu.Unlock()
	return nil
}

// RunTaskNow 立即触发一次任务。
func (s *Service) RunTaskNow(ctx context.Context, jobID string) (*ExecutionResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.repository.GetCronJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	s.loggerFor(ctx).Info("手动触发自动化任务",
		"job_id", job.JobID,
		"agent_id", job.AgentID,
	)
	return s.startJobExecution(ctx, *job, "manual", s.nowFn())
}

// ListTaskRuns 返回任务运行历史。
func (s *Service) ListTaskRuns(ctx context.Context, jobID string) ([]CronRun, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	job, err := s.repository.GetCronJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	return s.repository.ListRunsByJob(ctx, job.JobID)
}

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job CronJob) error {
	if strings.TrimSpace(job.SessionTarget.Kind) != SessionTargetIsolated {
		return nil
	}
	workspacePath, err := s.resolveAutomationWorkspacePath(ctx, job.AgentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	prefix := fmt.Sprintf("agent:%s:automation:dm:cron:%s:", strings.TrimSpace(job.AgentID), strings.TrimSpace(job.JobID))
	files := workspacestore.NewSessionFileStore(s.config.WorkspacePath)
	sessions, err := files.ListSessions(workspacePath)
	if err != nil {
		return err
	}
	for _, item := range sessions {
		sessionKey := strings.TrimSpace(item.SessionKey)
		if !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent || !parsed.IsStructured || parsed.Channel != "automation" {
			continue
		}
		if _, deleteErr := files.DeleteSession(workspacePath, sessionKey); deleteErr != nil {
			return deleteErr
		}
		if s.sessionCloser != nil {
			_ = s.sessionCloser.CloseSession(context.Background(), sessionKey)
		}
	}
	return nil
}

func (s *Service) resolveAutomationWorkspacePath(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(agentValue.WorkspacePath) != "" {
			return strings.TrimSpace(agentValue.WorkspacePath), nil
		}
	}
	return strings.TrimSpace(s.config.WorkspacePath), nil
}
