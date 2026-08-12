// INPUT: heartbeat 领取的 scheduled_task.trigger 事件、task-owned run 与 owner scope。
// OUTPUT: 使用原 job/run/revision 权限上下文串行下发到 Main Session，并完成 run ledger。
// POS: Main Session 定时任务执行桥；heartbeat 只负责唤醒和串行，不接管任务授权。
package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
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

func selectHeartbeatSystemEvents(items []automationdomain.SystemEvent) []automationdomain.SystemEvent {
	for _, item := range items {
		if isTaskBoundScheduledMainSessionEvent(item) {
			return []automationdomain.SystemEvent{item}
		}
	}
	return items
}

func scheduledMainSessionEvent(items []automationdomain.SystemEvent) (automationdomain.SystemEvent, bool) {
	if len(items) != 1 || !isTaskBoundScheduledMainSessionEvent(items[0]) {
		return automationdomain.SystemEvent{}, false
	}
	return items[0], true
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
		s.failScheduledMainSessionEvent(ctx, agentID, event, nil, payload.RunID, errors.New("scheduled main-session event owner does not match heartbeat owner"))
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
	started, err := s.repository.StartQueuedMainRun(ctx, ownerUserID, run.RunID, roundID, startedAt)
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
	s.markScheduledHeartbeatStarted(ctx, agentID, startedAt)
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
			finishedAt := s.nowFn()
			s.finishHeartbeatRuntime(agentID, &startedAt, &finishedAt, nil)
			s.continuePendingSystemEvents(agentID)
		},
		resumeAttempt,
	)
}

func (s *Service) markScheduledHeartbeatStarted(ctx context.Context, agentID string, startedAt time.Time) {
	s.mu.Lock()
	if runtime := s.heartbeatState[strings.TrimSpace(agentID)]; runtime != nil {
		runtime.LastHeartbeatAt = cloneTimePointer(&startedAt)
		runtime.DeliveryError = nil
	}
	s.mu.Unlock()
	_ = s.persistHeartbeatTimes(ctx, agentID, &startedAt, nil)
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
		finishedAt := s.nowFn()
		message := errorPointer(runErr)
		finished, finishErr := s.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
			RunID:        strings.TrimSpace(runID),
			Status:       automationdomain.RunStatusFailed,
			FinishedAt:   finishedAt,
			ErrorMessage: message,
		})
		if finishErr == nil && finished {
			s.finishJobRuntime(job.JobID, &finishedAt, automationdomain.RunStatusFailed, message)
		}
	}
	s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(runErr))
	s.loggerFor(ctx).Error("Main Session 定时任务事件处理失败",
		"agent_id", agentID,
		"event_id", event.EventID,
		"run_id", runID,
		"err", runErr,
	)
	s.continuePendingSystemEvents(agentID)
}

func (s *Service) continuePendingSystemEvents(agentID string) {
	items, err := s.repository.ListNewSystemEventsByAgent(context.Background(), strings.TrimSpace(agentID))
	if err != nil || len(items) == 0 {
		return
	}
	go s.dispatchHeartbeat(strings.TrimSpace(agentID), "pending-system-event")
}
