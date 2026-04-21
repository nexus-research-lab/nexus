// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service.go
// @Date   ：2026/04/11 15:28:00
// @Author ：leemysw
// 2026/04/11 15:28:00   Create
// =====================================================

package automation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/chat"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"
)

type chatRunner interface {
	HandleChat(context.Context, chatsvc.Request) error
}

type roomRunner interface {
	HandleChat(context.Context, roomsvc.ChatRequest) error
}

type workspaceReader interface {
	GetFile(context.Context, string, string) (*workspacepkg.FileContent, error)
}

type deliveryRouter interface {
	DeliverText(context.Context, string, string, channels.DeliveryTarget) (channels.DeliveryTarget, error)
}

type runtimeSessionCloser interface {
	CloseSession(context.Context, string) error
}

type jobRuntimeState struct {
	Job           CronJob
	Running       bool
	NextRunAt     *time.Time
	LastRunAt     *time.Time
	FailureStreak int
}

// 失败重试策略：连续失败不超过 maxFailureRetries 次时，按 retryBackoffs 顺序退避重排。
// 超过阈值后回退到 Schedule 的正常节奏，由下一个自然触发继续尝试。
var retryBackoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

func retryBackoffFor(streak int) (time.Duration, bool) {
	if streak <= 0 || streak > len(retryBackoffs) {
		return 0, false
	}
	return retryBackoffs[streak-1], true
}

type heartbeatRuntimeState struct {
	Config          HeartbeatConfig
	Running         bool
	PendingWake     bool
	NextRunAt       *time.Time
	LastHeartbeatAt *time.Time
	LastAckAt       *time.Time
	DeliveryError   *string
}

type heartbeatWakeRequest struct {
	AgentID    string
	SessionKey string
	WakeMode   string
	Text       string
}

const heartbeatExplicitTargetUnsupportedMessage = "heartbeat target_mode=explicit is not supported in Task 6 runtime"

// Service 提供 scheduled tasks 与 heartbeat 的真实业务能力。
type Service struct {
	config        config.Config
	repository    *sqlRepository
	agents        *agent2.Service
	chat          chatRunner
	room          roomRunner
	permission    *permissionctx.Context
	workspace     workspaceReader
	delivery      deliveryRouter
	logger        *slog.Logger
	sessionCloser runtimeSessionCloser

	nowFn     func() time.Time
	idFactory func(string) string

	mu             sync.Mutex
	jobStates      map[string]*jobRuntimeState
	heartbeatState map[string]*heartbeatRuntimeState
	wakeRequests   map[string][]heartbeatWakeRequest
	started        bool
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewService 创建自动化服务。
func NewService(
	cfg config.Config,
	db *sql.DB,
	agents *agent2.Service,
	chat chatRunner,
	room roomRunner,
	permission *permissionctx.Context,
	workspace workspaceReader,
	delivery deliveryRouter,
) *Service {
	return &Service{
		config:         cfg,
		repository:     NewRepository(cfg, db),
		agents:         agents,
		chat:           chat,
		room:           room,
		permission:     permission,
		workspace:      workspace,
		delivery:       delivery,
		logger:         logx.NewDiscardLogger(),
		nowFn:          func() time.Time { return time.Now().UTC() },
		idFactory:      newAutomationID,
		jobStates:      make(map[string]*jobRuntimeState),
		heartbeatState: make(map[string]*heartbeatRuntimeState),
		wakeRequests:   make(map[string][]heartbeatWakeRequest),
	}
}

// SetLogger 注入业务日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetRuntimeSessionCloser 注入运行时会话关闭器，用于清理 isolated 自动化会话。
func (s *Service) SetRuntimeSessionCloser(sessionCloser runtimeSessionCloser) {
	s.sessionCloser = sessionCloser
}

// Start 启动后台调度循环。
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if s.agents != nil {
		if err := s.agents.EnsureReady(ctx); err != nil {
			return err
		}
	}
	if err := s.bootstrapRuntime(ctx); err != nil {
		return err
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	s.loggerFor(ctx).Info("自动化调度器已启动")
	go s.runLoop(loopCtx)
	return nil
}

// Stop 停止后台调度循环。
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	s.loggerFor(context.Background()).Info("自动化调度器已停止")
}

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

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
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

// GetHeartbeatStatus 返回 heartbeat 状态。
func (s *Service) GetHeartbeatStatus(ctx context.Context, agentID string) (*HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return &HeartbeatStatus{
		AgentID:         state.Config.AgentID,
		Enabled:         state.Config.Enabled,
		EverySeconds:    state.Config.EverySeconds,
		TargetMode:      state.Config.TargetMode,
		AckMaxChars:     state.Config.AckMaxChars,
		Running:         state.Running,
		PendingWake:     state.PendingWake,
		NextRunAt:       cloneTimePointer(state.NextRunAt),
		LastHeartbeatAt: cloneTimePointer(state.LastHeartbeatAt),
		LastAckAt:       cloneTimePointer(state.LastAckAt),
		DeliveryError:   cloneStringPointer(state.DeliveryError),
	}, nil
}

// UpdateHeartbeat 更新 heartbeat 配置。
func (s *Service) UpdateHeartbeat(ctx context.Context, agentID string, input HeartbeatUpdateInput) (*HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	configValue := HeartbeatConfig{
		AgentID:      strings.TrimSpace(agentID),
		Enabled:      input.Enabled,
		EverySeconds: input.EverySeconds,
		TargetMode:   strings.TrimSpace(input.TargetMode),
		AckMaxChars:  input.AckMaxChars,
	}.Normalized()
	if configValue.TargetMode == HeartbeatTargetExplicit {
		return nil, ErrHeartbeatConfigInvalid
	}
	if err := configValue.Validate(); err != nil {
		return nil, err
	}

	state, err := s.ensureHeartbeatState(ctx, configValue.AgentID)
	if err != nil {
		return nil, err
	}
	state.Config = configValue
	state.NextRunAt = s.computeHeartbeatNext(configValue, s.nowFn())
	state.DeliveryError = nil
	if !configValue.Enabled {
		state.PendingWake = false
		state.Running = false
	}
	if err = s.repository.UpsertHeartbeatState(
		ctx,
		s.idFactory("hb"),
		configValue,
		state.LastHeartbeatAt,
		state.LastAckAt,
	); err != nil {
		return nil, err
	}
	return s.GetHeartbeatStatus(ctx, configValue.AgentID)
}

// WakeHeartbeat 手动登记一次 heartbeat 唤醒。
func (s *Service) WakeHeartbeat(ctx context.Context, agentID string, request HeartbeatWakeRequest) (*HeartbeatWakeResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = WakeModeNow
	}
	if mode != WakeModeNow && mode != WakeModeNextHeartbeat {
		return nil, errors.New("mode must be one of now, next-heartbeat")
	}

	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if request.Text != nil && strings.TrimSpace(*request.Text) != "" {
		if err = s.repository.InsertSystemEvent(
			ctx,
			s.idFactory("evt"),
			"heartbeat.wake",
			"heartbeat",
			state.Config.AgentID,
			map[string]any{
				"agent_id":  state.Config.AgentID,
				"text":      strings.TrimSpace(*request.Text),
				"wake_mode": mode,
			},
		); err != nil {
			return nil, err
		}
	}
	sessionKey := buildMainSessionKey(state.Config.AgentID)
	s.recordWakeRequest(state.Config.AgentID, sessionKey, mode, request.Text)

	s.mu.Lock()
	switch mode {
	case WakeModeNow:
		if state.Running {
			state.PendingWake = true
			s.mu.Unlock()
			return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
		}
		state.PendingWake = true
		s.mu.Unlock()
		s.dispatchHeartbeat(state.Config.AgentID, "wake-now")
		return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
	default:
		state.PendingWake = true
		s.mu.Unlock()
		return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: false}, nil
	}
}

func (s *Service) bootstrapRuntime(ctx context.Context) error {
	jobs, err := s.repository.ListCronJobs(ctx, "")
	if err != nil {
		return err
	}
	for _, item := range jobs {
		s.ensureJobState(item)
	}

	configs, err := s.repository.ListEnabledHeartbeatStates(ctx)
	if err != nil {
		return err
	}
	for _, item := range configs {
		if _, stateErr := s.ensureHeartbeatState(ctx, item.AgentID); stateErr != nil {
			return stateErr
		}
	}
	return nil
}

func (s *Service) runLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueOnce()
		}
	}
}

func (s *Service) runDueOnce() {
	now := s.nowFn()

	dueJobs := make([]CronJob, 0)
	dueHeartbeats := make([]string, 0)

	s.mu.Lock()
	for _, state := range s.jobStates {
		if state == nil || !state.Job.Enabled || state.Running || state.NextRunAt == nil {
			continue
		}
		if !state.NextRunAt.After(now) {
			dueJobs = append(dueJobs, state.Job)
		}
	}
	for agentID, state := range s.heartbeatState {
		if state == nil || state.Running {
			continue
		}
		if !state.Config.Enabled {
			continue
		}
		if s.hasImmediateWakeRequestLocked(agentID) {
			dueHeartbeats = append(dueHeartbeats, agentID)
			continue
		}
		if state.NextRunAt == nil || state.NextRunAt.After(now) {
			continue
		}
		dueHeartbeats = append(dueHeartbeats, agentID)
	}
	s.mu.Unlock()

	for _, item := range dueJobs {
		jobValue := item
		go func() {
			if _, err := s.startJobExecution(context.Background(), jobValue, "cron", now); err != nil {
				s.loggerFor(context.Background()).Error("定时任务触发失败",
					"job_id", jobValue.JobID,
					"agent_id", jobValue.AgentID,
					"trigger_kind", "cron",
					"err", err,
				)
				s.finishJobRuntime(jobValue.JobID, &now, false)
			}
		}()
	}
	for _, agentID := range dueHeartbeats {
		go s.dispatchHeartbeat(agentID, "heartbeat")
	}
}

func (s *Service) startJobExecution(ctx context.Context, job CronJob, triggerKind string, scheduledFor time.Time) (*ExecutionResult, error) {
	logger := s.loggerFor(ctx).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"trigger_kind", triggerKind,
	)
	if err := s.ensureDirectTargetSupported(job.SessionTarget); err != nil {
		logger.Error("自动化任务目标校验失败", "err", err)
		return nil, err
	}
	if strings.TrimSpace(job.SessionTarget.Kind) == SessionTargetMain {
		eventID, err := s.enqueueMainSessionEvent(ctx, job, triggerKind)
		if err != nil {
			return nil, err
		}
		mode := job.SessionTarget.WakeMode
		if mode == "" {
			mode = WakeModeNextHeartbeat
		}
		if _, err := s.WakeHeartbeat(ctx, job.AgentID, HeartbeatWakeRequest{Mode: mode}); err != nil {
			_ = s.repository.MarkSystemEventStatus(context.Background(), eventID, "failed")
			logger.Error("自动化任务唤醒主会话 heartbeat 失败", "err", err)
			return nil, err
		}
		sessionKey, err := resolveSessionKey(job, nil)
		if err != nil {
			logger.Error("自动化任务解析主会话键失败", "err", err)
			return nil, err
		}
		logger.Info("自动化任务已排入主会话",
			"session_key", sessionKey,
			"wake_mode", mode,
		)
		return &ExecutionResult{
			JobID:        job.JobID,
			Status:       RunStatusQueuedToMain,
			SessionKey:   sessionKey,
			ScheduledFor: cloneTimePointer(&scheduledFor),
		}, nil
	}

	state := s.ensureJobState(job)
	s.mu.Lock()
	if state.Running {
		s.mu.Unlock()
		logger.Warn("自动化任务已在运行中")
		return nil, errors.New("任务正在运行中")
	}
	state.Running = true
	s.mu.Unlock()

	runID := s.idFactory("run")
	sessionKey, err := resolveSessionKey(job, &runID)
	if err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		logger.Error("自动化任务解析执行会话键失败", "run_id", runID, "err", err)
		return nil, err
	}
	if err := s.repository.InsertRunPending(ctx, runID, job.JobID, &scheduledFor); err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		return nil, err
	}
	if err := s.repository.MarkRunRunning(ctx, runID, s.nowFn()); err != nil {
		s.finishJobRuntime(job.JobID, nil, false)
		return nil, err
	}

	roundID := s.idFactory("round")
	logger.Info("开始执行自动化任务",
		"run_id", runID,
		"round_id", roundID,
		"session_key", sessionKey,
	)
	sink := newExecutionSink("automation:" + runID)
	cleanup := s.bindSink(sessionKey, sink)
	if err := s.dispatchToSession(ctx, sessionKey, roundID, job.AgentID, job.Instruction); err != nil {
		cleanup()
		sink.Close()
		finishedAt := s.nowFn()
		message := err.Error()
		_ = s.repository.MarkRunFinished(context.Background(), runID, RunStatusFailed, finishedAt, &message)
		s.finishJobRuntime(job.JobID, &finishedAt, false)
		logger.Error("自动化任务下发失败",
			"run_id", runID,
			"round_id", roundID,
			"session_key", sessionKey,
			"err", err,
		)
		return nil, err
	}

	go s.observeJobRun(job, runID, roundID, sessionKey, sink, cleanup)

	return &ExecutionResult{
		JobID:        job.JobID,
		RunID:        &runID,
		Status:       RunStatusRunning,
		SessionKey:   sessionKey,
		ScheduledFor: cloneTimePointer(&scheduledFor),
		RoundID:      &roundID,
		MessageCount: 0,
	}, nil
}

func (s *Service) observeJobRun(
	job CronJob,
	runID string,
	roundID string,
	sessionKey string,
	sink *executionSink,
	cleanup func(),
) {
	defer cleanup()
	defer sink.Close()

	waitCtx, cancel := context.WithTimeout(context.Background(), waitTimeout(0))
	defer cancel()
	observation := sink.WaitForRound(waitCtx, roundID)

	finishedAt := s.nowFn()
	status := observation.Status
	if status == "" {
		status = RunStatusFailed
	}
	errorMessage := cloneStringPointer(observation.ErrorMessage)
	if status == RunStatusSucceeded {
		if deliveryError := s.deliverJobObservation(job, sessionKey, observation); deliveryError != nil {
			status = RunStatusFailed
			errorMessage = deliveryError
		}
	}
	logger := s.loggerFor(context.Background()).With(
		"job_id", job.JobID,
		"agent_id", job.AgentID,
		"run_id", runID,
		"round_id", roundID,
	)
	if errorMessage != nil {
		logger.Error("自动化任务执行结束",
			"status", status,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
			"err", *errorMessage,
		)
	} else {
		logger.Info("自动化任务执行结束",
			"status", status,
			"message_count", observation.MessageCount,
			"session_id", anyStringPointer(observation.SessionID),
		)
	}
	_ = s.repository.MarkRunFinished(context.Background(), runID, status, finishedAt, errorMessage)
	s.finishJobRuntime(job.JobID, &finishedAt, status == RunStatusSucceeded)
}

func (s *Service) dispatchHeartbeat(agentID string, reason string) {
	ctx := context.Background()
	logger := s.loggerFor(ctx).With("agent_id", agentID, "reason", reason)
	sessionKey := buildMainSessionKey(agentID)
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		logger.Error("heartbeat 状态初始化失败", "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		return
	}

	s.mu.Lock()
	if runtime := s.heartbeatState[agentID]; runtime != nil {
		if runtime.Running {
			s.mu.Unlock()
			logger.Warn("heartbeat 已在运行中，跳过重复触发")
			return
		}
		runtime.Running = true
		runtime.PendingWake = false
		state = runtime
	}
	s.mu.Unlock()
	immediateWakeRequests, deferredWakeRequests := s.takeWakeRequests(agentID, sessionKey)

	events, err := s.claimSystemEvents(ctx, agentID)
	if err != nil {
		logger.Error("heartbeat 拉取系统事件失败", "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		return
	}

	instruction, err := s.buildHeartbeatInstruction(ctx, agentID, events, immediateWakeRequests, deferredWakeRequests)
	if err != nil {
		logger.Error("heartbeat 构建指令失败", "event_count", len(events), "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		s.failEvents(events)
		return
	}
	if strings.TrimSpace(instruction) == "" {
		logger.Info("heartbeat 无可执行内容", "event_count", len(events))
		s.markEventsProcessed(events)
		s.finishHeartbeatRuntime(agentID, nil, nil, nil)
		return
	}

	roundID := s.idFactory("hbround")
	sink := newExecutionSink("heartbeat:" + agentID + ":" + roundID)
	cleanup := s.bindSink(sessionKey, sink)
	if err = s.dispatchToSession(ctx, sessionKey, roundID, agentID, instruction); err != nil {
		cleanup()
		sink.Close()
		s.failEvents(events)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		logger.Error("heartbeat 下发失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"event_count", len(events),
			"err", err,
		)
		return
	}
	logger.Info("heartbeat 已下发",
		"session_key", sessionKey,
		"round_id", roundID,
		"event_count", len(events),
	)

	startedAt := s.nowFn()
	s.mu.Lock()
	if runtime := s.heartbeatState[agentID]; runtime != nil {
		runtime.LastHeartbeatAt = cloneTimePointer(&startedAt)
		runtime.DeliveryError = nil
	}
	s.mu.Unlock()
	_ = s.persistHeartbeatTimes(ctx, agentID, &startedAt, nil)

	go func() {
		defer cleanup()
		defer sink.Close()

		waitCtx, cancel := context.WithTimeout(context.Background(), waitTimeout(0))
		defer cancel()
		observation := sink.WaitForRound(waitCtx, roundID)
		if observation.Status == RunStatusSucceeded {
			finishedAt := s.nowFn()
			s.markEventsProcessed(events)
			deliveryError := s.deliverHeartbeatObservation(agentID, state.Config, observation)
			if deliveryError != nil {
				logger.Error("heartbeat 执行完成但投递失败",
					"status", observation.Status,
					"message_count", observation.MessageCount,
					"delivery_error", *deliveryError,
				)
			} else {
				logger.Info("heartbeat 执行成功",
					"status", observation.Status,
					"message_count", observation.MessageCount,
				)
			}
			s.finishHeartbeatRuntime(agentID, &startedAt, &finishedAt, deliveryError)
			return
		}
		s.failEvents(events)
		if observation.ErrorMessage != nil {
			logger.Error("heartbeat 执行失败",
				"status", observation.Status,
				"message_count", observation.MessageCount,
				"err", *observation.ErrorMessage,
			)
		} else {
			logger.Error("heartbeat 执行失败",
				"status", observation.Status,
				"message_count", observation.MessageCount,
			)
		}
		s.finishHeartbeatRuntime(agentID, &startedAt, nil, observation.ErrorMessage)
	}()

	_ = reason
}

func (s *Service) buildHeartbeatInstruction(
	ctx context.Context,
	agentID string,
	events []SystemEvent,
	immediateWakeRequests []heartbeatWakeRequest,
	deferredWakeRequests []heartbeatWakeRequest,
) (string, error) {
	sections := make([]string, 0, 3)
	if s.workspace != nil {
		file, err := s.workspace.GetFile(ctx, agentID, "HEARTBEAT.md")
		if err != nil && !errors.Is(err, workspacepkg.ErrFileNotFound) {
			return "", err
		}
		if file != nil && strings.TrimSpace(file.Content) != "" {
			tasks := parseHeartbeatTasks(file.Content)
			if len(tasks) > 0 {
				taskLines := make([]string, 0, len(tasks))
				for _, item := range tasks {
					line := firstNonEmpty(
						strings.TrimSpace(item.Prompt),
						strings.TrimSpace(item.Name),
						strings.TrimSpace(item.Interval),
					)
					if line != "" {
						taskLines = append(taskLines, line)
					}
				}
				if len(taskLines) > 0 {
					sections = append(sections, "Heartbeat tasks:\n- "+strings.Join(taskLines, "\n- "))
				}
			} else {
				sections = append(sections, strings.TrimSpace(file.Content))
			}
		}
	}

	eventLines := make([]string, 0, len(events))
	for _, item := range events {
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(item.Payload), &payload)
		text := strings.TrimSpace(anyString(payload["text"]))
		if text != "" {
			eventLines = append(eventLines, text)
			continue
		}
		eventLines = append(eventLines, item.EventType)
	}
	if len(eventLines) > 0 {
		sections = append(sections, "System events:\n- "+strings.Join(eventLines, "\n- "))
	}

	existingLines := make(map[string]struct{}, len(eventLines))
	for _, item := range eventLines {
		existingLines[item] = struct{}{}
	}
	wakeLines := make([]string, 0, len(immediateWakeRequests)+len(deferredWakeRequests))
	appendWakeLine := func(request heartbeatWakeRequest) {
		text := strings.TrimSpace(request.Text)
		if text != "" {
			if _, duplicated := existingLines[text]; duplicated {
				return
			}
			wakeLines = append(wakeLines, text)
			existingLines[text] = struct{}{}
			return
		}
		fallback := "wake request (" + strings.TrimSpace(request.WakeMode) + ")"
		if strings.TrimSpace(request.WakeMode) == "" {
			fallback = "wake request (unknown)"
		}
		if _, duplicated := existingLines[fallback]; duplicated {
			return
		}
		wakeLines = append(wakeLines, fallback)
		existingLines[fallback] = struct{}{}
	}
	for _, item := range immediateWakeRequests {
		appendWakeLine(item)
	}
	for _, item := range deferredWakeRequests {
		appendWakeLine(item)
	}
	if len(wakeLines) > 0 {
		sections = append(sections, "Wake requests:\n- "+strings.Join(wakeLines, "\n- "))
	}
	if summary := s.describeScheduledTasksSection(ctx, agentID); summary != "" {
		sections = append(sections, summary)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n")), nil
}

func (s *Service) claimSystemEvents(ctx context.Context, agentID string) ([]SystemEvent, error) {
	items, err := s.repository.ListNewSystemEventsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if markErr := s.repository.MarkSystemEventStatus(ctx, item.EventID, "processing"); markErr != nil {
			return nil, markErr
		}
	}
	return items, nil
}

func (s *Service) markEventsProcessed(items []SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "processed")
	}
}

func (s *Service) failEvents(items []SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "failed")
	}
}

func (s *Service) bindSink(sessionKey string, sink *executionSink) func() {
	if s.permission == nil {
		return func() {}
	}
	s.permission.BindSession(sessionKey, sink, sink.Key(), false)
	return func() {
		s.permission.UnbindSession(sessionKey, sink)
	}
}

func (s *Service) dispatchToSession(ctx context.Context, sessionKey string, roundID string, agentID string, instruction string) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(ctx, roomsvc.ChatRequest{
			SessionKey:     sessionKey,
			ConversationID: parsed.ConversationID,
			Content:        instruction,
			RoundID:        roundID,
			ReqID:          roundID,
		})
	}
	if s.chat == nil {
		return errors.New("automation chat runner is not configured")
	}
	return s.chat.HandleChat(ctx, chatsvc.Request{
		SessionKey: sessionKey,
		AgentID:    firstNonEmpty(agentID, parsed.AgentID),
		Content:    instruction,
		RoundID:    roundID,
		ReqID:      roundID,
	})
}

func (s *Service) enqueueMainSessionEvent(ctx context.Context, job CronJob, triggerKind string) (string, error) {
	eventID := s.idFactory("evt")
	if err := s.repository.InsertSystemEvent(
		ctx,
		eventID,
		"cron.trigger",
		"cron",
		job.AgentID,
		map[string]any{
			"agent_id":            job.AgentID,
			"job_id":              job.JobID,
			"text":                job.Instruction,
			"trigger_kind":        triggerKind,
			"session_target_kind": job.SessionTarget.Kind,
		},
	); err != nil {
		return "", err
	}
	return eventID, nil
}

func (s *Service) ensureReady(ctx context.Context) error {
	if s.agents == nil {
		return nil
	}
	return s.agents.EnsureReady(ctx)
}

func (s *Service) requireAgent(ctx context.Context, agentID string) (*agent2.Agent, error) {
	if s.agents == nil {
		return nil, nil
	}
	return s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
}

func (s *Service) validateAgentAndTarget(ctx context.Context, agentID string, target SessionTarget) error {
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kind) != SessionTargetBound {
		return nil
	}
	parsed := protocol.ParseSessionKey(target.BoundSessionKey)
	if parsed.Kind == protocol.SessionKeyKindAgent && parsed.AgentID != "" && parsed.AgentID != strings.TrimSpace(agentID) {
		return errors.New("agent_id 与 session_target 不一致")
	}
	return nil
}

func (s *Service) ensureDirectTargetSupported(target SessionTarget) error {
	if strings.TrimSpace(target.Kind) == SessionTargetMain {
		return nil
	}
	sessionKey, err := resolveSessionKey(CronJob{
		AgentID:       "noop",
		SessionTarget: target,
	}, stringPointer("noop"))
	if err != nil {
		return err
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return errors.New("shared room session automation 暂不支持")
	}
	return nil
}

func (s *Service) ensureJobState(job CronJob) *jobRuntimeState {
	s.mu.Lock()
	state := s.jobStates[job.JobID]
	if state == nil {
		state = &jobRuntimeState{}
		s.jobStates[job.JobID] = state
	}
	state.Job = job
	if state.NextRunAt == nil || !job.Enabled {
		state.NextRunAt = s.computeJobNext(job, s.nowFn())
	}
	if !job.Enabled {
		state.Running = false
	}
	// 启动期发现 at-kind 已过期且仍处于启用态时，主动落库为停用，避免反复检查空 NextRunAt 浪费循环。
	shouldDisable := job.Enabled &&
		strings.EqualFold(job.Schedule.Kind, ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
	return state
}

func (s *Service) ensureHeartbeatState(ctx context.Context, agentID string) (*heartbeatRuntimeState, error) {
	s.mu.Lock()
	state := s.heartbeatState[strings.TrimSpace(agentID)]
	s.mu.Unlock()
	if state != nil {
		return state, nil
	}

	configValue, lastHeartbeatAt, lastAckAt, err := s.repository.GetHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if configValue == nil {
		defaultValue := DefaultHeartbeatConfig(agentID)
		sanitizedConfig, deliveryError := sanitizeHeartbeatConfig(defaultValue)
		state = &heartbeatRuntimeState{
			Config:          sanitizedConfig,
			NextRunAt:       s.computeHeartbeatNext(sanitizedConfig, s.nowFn()),
			LastHeartbeatAt: cloneTimePointer(lastHeartbeatAt),
			LastAckAt:       cloneTimePointer(lastAckAt),
			DeliveryError:   cloneStringPointer(deliveryError),
		}
	} else {
		normalized, deliveryError := sanitizeHeartbeatConfig(configValue.Normalized())
		state = &heartbeatRuntimeState{
			Config:          normalized,
			NextRunAt:       s.computeHeartbeatNext(normalized, s.nowFn()),
			LastHeartbeatAt: cloneTimePointer(lastHeartbeatAt),
			LastAckAt:       cloneTimePointer(lastAckAt),
			DeliveryError:   cloneStringPointer(deliveryError),
		}
	}

	s.mu.Lock()
	s.heartbeatState[state.Config.AgentID] = state
	s.mu.Unlock()
	return state, nil
}

func (s *Service) computeJobNext(job CronJob, now time.Time) *time.Time {
	if !job.Enabled {
		return nil
	}
	next, err := ComputeNextRunAt(job.Schedule, now)
	if err != nil {
		return nil
	}
	return next
}

func (s *Service) computeHeartbeatNext(configValue HeartbeatConfig, now time.Time) *time.Time {
	if !configValue.Enabled {
		return nil
	}
	next := now.UTC().Add(time.Duration(configValue.EverySeconds) * time.Second)
	return &next
}

func (s *Service) finishJobRuntime(jobID string, finishedAt *time.Time, succeeded bool) {
	s.mu.Lock()
	state := s.jobStates[jobID]
	if state == nil {
		s.mu.Unlock()
		return
	}
	state.Running = false
	if finishedAt != nil {
		state.LastRunAt = cloneTimePointer(finishedAt)
	}

	now := s.nowFn()
	naturalNext := s.computeJobNext(state.Job, now)

	if succeeded {
		state.FailureStreak = 0
		state.NextRunAt = naturalNext
	} else {
		state.FailureStreak++
		state.NextRunAt = naturalNext
		if backoff, ok := retryBackoffFor(state.FailureStreak); ok {
			retryAt := now.UTC().Add(backoff)
			if naturalNext == nil || retryAt.Before(*naturalNext) {
				retryCopy := retryAt
				state.NextRunAt = &retryCopy
			}
		}
	}

	// at-kind 是一次性任务：成功或重试耗尽后没有下一次自然触发，主动停用以避免数据库残留启用态。
	shouldDisable := state.Job.Enabled &&
		strings.EqualFold(state.Job.Schedule.Kind, ScheduleKindAt) &&
		state.NextRunAt == nil
	jobSnapshot := state.Job
	s.mu.Unlock()

	if shouldDisable {
		s.disableExpiredJobAsync(jobSnapshot)
	}
}

func (s *Service) disableExpiredJobAsync(job CronJob) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		updated := job
		updated.Enabled = false
		if _, err := s.repository.UpsertCronJob(context.Background(), updated); err != nil {
			s.loggerFor(context.Background()).Warn("at 任务到期自动停用失败",
				"job_id", job.JobID,
				"agent_id", job.AgentID,
				"err", err,
			)
			return
		}
		s.mu.Lock()
		if state := s.jobStates[job.JobID]; state != nil {
			state.Job.Enabled = false
		}
		s.mu.Unlock()
		s.loggerFor(context.Background()).Info("at 任务到期已自动停用",
			"job_id", job.JobID,
			"agent_id", job.AgentID,
		)
	}()
}

func (s *Service) finishHeartbeatRuntime(agentID string, startedAt *time.Time, ackAt *time.Time, deliveryError *string) {
	s.mu.Lock()
	state := s.heartbeatState[strings.TrimSpace(agentID)]
	if state != nil {
		state.Running = false
		state.NextRunAt = s.computeHeartbeatNext(state.Config, s.nowFn())
		if startedAt != nil {
			state.LastHeartbeatAt = cloneTimePointer(startedAt)
		}
		if ackAt != nil {
			state.LastAckAt = cloneTimePointer(ackAt)
		}
		state.DeliveryError = cloneStringPointer(deliveryError)
	}
	configValue := HeartbeatConfig{}
	lastHeartbeatAt := (*time.Time)(nil)
	lastAckAt := (*time.Time)(nil)
	if state != nil {
		configValue = state.Config
		lastHeartbeatAt = cloneTimePointer(state.LastHeartbeatAt)
		lastAckAt = cloneTimePointer(state.LastAckAt)
	}
	s.mu.Unlock()

	if configValue.AgentID != "" {
		_ = s.repository.UpsertHeartbeatState(context.Background(), s.idFactory("hb"), configValue, lastHeartbeatAt, lastAckAt)
	}
}

func (s *Service) persistHeartbeatTimes(ctx context.Context, agentID string, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return err
	}
	return s.repository.UpsertHeartbeatState(ctx, s.idFactory("hb"), state.Config, lastHeartbeatAt, lastAckAt)
}

func (s *Service) recordWakeRequest(agentID string, sessionKey string, wakeMode string, text *string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionKey = strings.TrimSpace(sessionKey)
	request := heartbeatWakeRequest{
		AgentID:    strings.TrimSpace(agentID),
		SessionKey: sessionKey,
		WakeMode:   strings.TrimSpace(wakeMode),
		Text:       strings.TrimSpace(anyStringPointer(text)),
	}
	s.wakeRequests[sessionKey] = append(s.wakeRequests[sessionKey], request)
	if state := s.heartbeatState[request.AgentID]; state != nil {
		state.PendingWake = true
	}
}

func (s *Service) hasImmediateWakeRequestLocked(agentID string) bool {
	sessionKey := buildMainSessionKey(agentID)
	for _, item := range s.wakeRequests[sessionKey] {
		if strings.TrimSpace(item.AgentID) == strings.TrimSpace(agentID) && item.WakeMode == WakeModeNow {
			return true
		}
	}
	return false
}

func (s *Service) takeWakeRequests(agentID string, sessionKey string) ([]heartbeatWakeRequest, []heartbeatWakeRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionKey = strings.TrimSpace(sessionKey)
	items := append([]heartbeatWakeRequest(nil), s.wakeRequests[sessionKey]...)
	delete(s.wakeRequests, sessionKey)

	immediate := make([]heartbeatWakeRequest, 0, len(items))
	deferred := make([]heartbeatWakeRequest, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		switch item.WakeMode {
		case WakeModeNow:
			immediate = append(immediate, item)
		case WakeModeNextHeartbeat:
			deferred = append(deferred, item)
		}
	}
	return immediate, deferred
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

func sanitizeHeartbeatConfig(configValue HeartbeatConfig) (HeartbeatConfig, *string) {
	result := configValue
	if strings.TrimSpace(result.TargetMode) != HeartbeatTargetExplicit {
		return result, nil
	}
	result.TargetMode = HeartbeatTargetNone
	return result, stringPointer(heartbeatExplicitTargetUnsupportedMessage)
}

func resolveSessionKey(job CronJob, runID *string) (string, error) {
	switch strings.TrimSpace(job.SessionTarget.Kind) {
	case SessionTargetMain:
		return buildMainSessionKey(job.AgentID), nil
	case SessionTargetBound:
		return strings.TrimSpace(job.SessionTarget.BoundSessionKey), nil
	case SessionTargetNamed:
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", strings.TrimSpace(job.SessionTarget.NamedSessionKey), ""), nil
	default:
		if runID == nil || strings.TrimSpace(*runID) == "" {
			return "", errors.New("isolated target requires run_id")
		}
		return protocol.BuildAgentSessionKey(job.AgentID, "automation", "dm", fmt.Sprintf("cron:%s:%s", job.JobID, strings.TrimSpace(*runID)), ""), nil
	}
}

func buildMainSessionKey(agentID string) string {
	return protocol.BuildAgentSessionKey(strings.TrimSpace(agentID), "automation", "dm", "main", "")
}

func newAutomationID(prefix string) string {
	buffer := make([]byte, 10)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), time.Now().UnixNano())
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(buffer)
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

func errorPointer(err error) *string {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	return &message
}

func anyStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func (s *Service) deliverJobObservation(
	job CronJob,
	executionSessionKey string,
	observation executionObservation,
) *string {
	if strings.TrimSpace(job.Delivery.Mode) == "" || strings.TrimSpace(job.Delivery.Mode) == DeliveryModeNone {
		return nil
	}
	if strings.TrimSpace(job.Delivery.Mode) == DeliveryModeExplicit &&
		strings.TrimSpace(job.Delivery.Channel) == "websocket" &&
		strings.TrimSpace(job.Delivery.To) != "" &&
		strings.TrimSpace(job.Delivery.To) == strings.TrimSpace(executionSessionKey) {
		return nil
	}
	if s.delivery == nil {
		return stringPointer("delivery router is not configured")
	}
	text := firstNonEmpty(strings.TrimSpace(observation.ResultText), strings.TrimSpace(observation.AssistantText))
	if text == "" {
		return nil
	}
	if _, err := s.delivery.DeliverText(
		context.Background(),
		job.AgentID,
		text,
		toChannelDeliveryTarget(job.Delivery),
	); err != nil {
		return errorPointer(err)
	}
	return nil
}

func (s *Service) deliverHeartbeatObservation(
	agentID string,
	configValue HeartbeatConfig,
	observation executionObservation,
) *string {
	if strings.TrimSpace(configValue.TargetMode) == "" || strings.TrimSpace(configValue.TargetMode) == HeartbeatTargetNone {
		return nil
	}
	if s.delivery == nil {
		return stringPointer("delivery router is not configured")
	}
	filtered := filterHeartbeatResponse(
		firstNonEmpty(strings.TrimSpace(observation.ResultText), strings.TrimSpace(observation.AssistantText)),
		configValue.AckMaxChars,
	)
	if !filtered.ShouldDeliver || strings.TrimSpace(filtered.Text) == "" {
		return nil
	}
	if _, err := s.delivery.DeliverText(
		context.Background(),
		agentID,
		filtered.Text,
		channels.DeliveryTarget{Mode: strings.TrimSpace(configValue.TargetMode)},
	); err != nil {
		return errorPointer(err)
	}
	return nil
}
