// INPUT: scheduled task 写入的 scheduled_task.trigger 事件、task-owned run 与 owner scope。
// OUTPUT: 写入即唤醒、按 Agent 串行领取并以原 job/run/revision 权限上下文下发到 Main Session。
// POS: Main Session 定时任务执行桥；派发器只负责唤醒和串行，不接管任务授权。
package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

const scheduledMainSessionEventType = "scheduled_task.trigger"

type scheduledMainSessionEventPayload struct {
	AgentID                         string `json:"agent_id"`
	JobID                           string `json:"job_id"`
	OwnerUserID                     string `json:"owner_user_id"`
	PermissionRequestID             string `json:"permission_request_id"`
	PermissionRequestPolicyRevision int    `json:"permission_request_policy_revision"`
	PolicyRevision                  int    `json:"policy_revision"`
	RunID                           string `json:"run_id"`
	TriggerKind                     string `json:"trigger_kind"`
}

// wakeMainSession 在事件落库后立即尝试派发；同 Agent 已有在途事件时由其收尾接力。
func (s *Service) wakeMainSession(agentID string) {
	go s.dispatchNextMainSessionEvent(strings.TrimSpace(agentID))
}

// dispatchNextMainSessionEvent 领取该 Agent 最早的一条任务事件并下发；没有可领取事件时释放串行位。
func (s *Service) dispatchNextMainSessionEvent(agentID string) {
	if agentID == "" || !s.acquireMainSession(agentID) {
		return
	}
	ctx, err := s.mainSessionOwnerContext(agentID)
	if err != nil {
		s.loggerFor(ctx).Error("Main Session 事件解析 Agent owner 失败", "agent_id", agentID, "err", err)
		s.releaseMainSession(agentID)
		return
	}
	event, ok, err := s.claimNextMainSessionEvent(ctx, agentID)
	if err != nil || !ok {
		if err != nil {
			s.loggerFor(ctx).Error("Main Session 事件领取失败", "agent_id", agentID, "err", err)
		}
		s.releaseMainSession(agentID)
		return
	}
	s.dispatchScheduledMainSessionEvent(ctx, agentID, automationexec.BuildMainSessionKey(agentID), event)
}

// finishMainSessionEvent 先释放串行位再重新领取，避免在途期间写入的事件丢失唤醒。
func (s *Service) finishMainSessionEvent(agentID string) {
	s.releaseMainSession(agentID)
	s.dispatchNextMainSessionEvent(strings.TrimSpace(agentID))
}

func (s *Service) acquireMainSession(agentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mainSessionBusy[agentID] {
		return false
	}
	s.mainSessionBusy[agentID] = true
	return true
}

func (s *Service) releaseMainSession(agentID string) {
	s.mu.Lock()
	delete(s.mainSessionBusy, strings.TrimSpace(agentID))
	s.mu.Unlock()
}

func (s *Service) mainSessionOwnerContext(agentID string) (context.Context, error) {
	ctx := context.Background()
	if s.agents == nil {
		return ctx, nil
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return ctx, err
	}
	if agentValue == nil || strings.TrimSpace(agentValue.OwnerUserID) == "" {
		return ctx, errors.New("Main Session Agent 缺少 owner_user_id")
	}
	return contextForOwner(ctx, agentValue.OwnerUserID), nil
}

// claimNextMainSessionEvent 按写入顺序 CAS 领取首个任务事件；不可解析的遗留行直接收口为 failed。
func (s *Service) claimNextMainSessionEvent(
	ctx context.Context,
	agentID string,
) (automationdomain.SystemEvent, bool, error) {
	items, err := s.repository.ListNewSystemEventsByAgent(ctx, agentID)
	if err != nil {
		return automationdomain.SystemEvent{}, false, err
	}
	for _, item := range items {
		if !isTaskBoundScheduledMainSessionEvent(item) {
			_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "failed")
			continue
		}
		claimed, claimErr := s.repository.ClaimSystemEvent(ctx, item.EventID)
		if claimErr != nil {
			return automationdomain.SystemEvent{}, false, claimErr
		}
		if claimed {
			item.Status = "processing"
			return item, true, nil
		}
	}
	return automationdomain.SystemEvent{}, false, nil
}

func isTaskBoundScheduledMainSessionEvent(event automationdomain.SystemEvent) bool {
	if strings.TrimSpace(event.EventType) != scheduledMainSessionEventType {
		return false
	}
	payload := scheduledMainSessionEventPayload{}
	if json.Unmarshal([]byte(event.Payload), &payload) != nil {
		return false
	}
	return strings.TrimSpace(payload.JobID) != "" && strings.TrimSpace(payload.RunID) != ""
}

func parseScheduledMainSessionEvent(event automationdomain.SystemEvent) (scheduledMainSessionEventPayload, error) {
	payload := scheduledMainSessionEventPayload{}
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return payload, fmt.Errorf("parse scheduled main-session event: %w", err)
	}
	payload.AgentID = strings.TrimSpace(payload.AgentID)
	payload.JobID = strings.TrimSpace(payload.JobID)
	payload.OwnerUserID = strings.TrimSpace(payload.OwnerUserID)
	payload.PermissionRequestID = strings.TrimSpace(payload.PermissionRequestID)
	payload.RunID = strings.TrimSpace(payload.RunID)
	payload.TriggerKind = strings.TrimSpace(payload.TriggerKind)
	if payload.JobID == "" || payload.RunID == "" {
		return payload, errors.New("scheduled main-session event is missing job_id or run_id")
	}
	return payload, nil
}

func (s *Service) dispatchScheduledMainSessionEvent(
	ctx context.Context,
	agentID string,
	sessionKey string,
	event automationdomain.SystemEvent,
) {
	payload, err := parseScheduledMainSessionEvent(event)
	if err != nil {
		s.failScheduledMainSessionEvent(ctx, agentID, event, nil, payload.RunID, err)
		return
	}
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if !scoped {
		ownerUserID = payload.OwnerUserID
		ctx = contextForOwner(ctx, ownerUserID)
	}
	if strings.TrimSpace(ownerUserID) == "" ||
		(payload.OwnerUserID != "" && payload.OwnerUserID != ownerUserID) {
		s.failScheduledMainSessionEvent(ctx, agentID, event, nil, payload.RunID, errors.New("scheduled main-session event owner does not match agent owner"))
		return
	}
	job, err := s.repository.GetScheduledTask(ctx, ownerUserID, payload.JobID)
	if err != nil || job == nil {
		if err == nil {
			err = automationdomain.ErrJobNotFound
		}
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, payload.RunID, err)
		return
	}
	if strings.TrimSpace(job.AgentID) != strings.TrimSpace(agentID) ||
		strings.TrimSpace(job.SessionTarget.Kind) != automationdomain.SessionTargetMain {
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, payload.RunID, errors.New("scheduled main-session event target no longer matches task"))
		return
	}
	if payload.PolicyRevision != job.PermissionPolicy.Revision {
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, payload.RunID, automationdomain.ErrPermissionRequestStale)
		return
	}
	run, err := s.repository.GetRun(ctx, ownerUserID, job.JobID, payload.RunID)
	if err != nil || run == nil {
		if err == nil {
			err = automationdomain.ErrRunNotFound
		}
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, payload.RunID, err)
		return
	}
	if run.PermissionPolicyRevision != job.PermissionPolicy.Revision {
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, run.RunID, automationdomain.ErrPermissionRequestStale)
		return
	}
	var resumeRequest *automationdomain.AutomationPermissionRequest
	if payload.TriggerKind == "permission_resume" {
		resumeRequest, err = s.loadPermissionResumeRequest(
			ctx,
			*job,
			*run,
			automationdomain.PermissionResumeInput{
				RequestID:      payload.PermissionRequestID,
				PolicyRevision: payload.PermissionRequestPolicyRevision,
			},
		)
		if err != nil {
			s.failScheduledMainSessionEvent(ctx, agentID, event, job, run.RunID, err)
			return
		}
	}

	startedAt := s.nowFn()
	roundID := s.idFactory("round")
	started, err := s.repository.StartQueuedMainRun(
		ctx,
		ownerUserID,
		run.RunID,
		payload.PermissionRequestID,
		roundID,
		startedAt,
	)
	if err != nil || !started {
		if err == nil {
			err = errors.New("scheduled main-session run is no longer queued")
		}
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, run.RunID, err)
		return
	}

	sink := automationexec.NewExecutionSink("automation:" + run.RunID)
	cleanup := s.bindSink(sessionKey, sink)
	completeAttempt := s.registerPhysicalAttempt(run.RunID, roundID)
	resumeAttempt := newPermissionResumeAttempt(resumeRequest)
	if err = s.dispatchJobToSession(ctx, *job, run.RunID, sessionKey, roundID, roomEventObserverForSink(sink), resumeAttempt); err != nil {
		completeAttempt()
		cleanup()
		sink.Close()
		s.failScheduledMainSessionEvent(ctx, agentID, event, job, run.RunID, err)
		return
	}
	_ = s.repository.MarkSystemEventStatus(context.Background(), event.EventID, "processed")
	s.loggerFor(ctx).Info("Main Session 定时任务已下发",
		"agent_id", agentID,
		"job_id", job.JobID,
		"run_id", run.RunID,
		"round_id", roundID,
	)
	go s.observeJobRunWithCompletion(
		*job,
		run.RunID,
		roundID,
		sessionKey,
		sink,
		cleanup,
		func() {
			completeAttempt()
			s.finishMainSessionEvent(agentID)
		},
		resumeAttempt,
	)
}

func (s *Service) failScheduledMainSessionEvent(
	ctx context.Context,
	agentID string,
	event automationdomain.SystemEvent,
	job *automationdomain.ScheduledTask,
	runID string,
	runErr error,
) {
	_ = s.repository.MarkSystemEventStatus(context.Background(), event.EventID, "failed")
	if job != nil && strings.TrimSpace(runID) != "" {
		_ = s.commitFailedRunTerminal(backgroundContextForJobOwner(*job), *job, runID, runErr)
	}
	s.loggerFor(ctx).Error("Main Session 定时任务事件处理失败",
		"agent_id", agentID,
		"event_id", event.EventID,
		"run_id", runID,
		"err", runErr,
	)
	s.finishMainSessionEvent(agentID)
}
