// INPUT: ScheduledTask、目标 DM/Room session、执行 sink 与调度权限上下文。
// OUTPUT: 标记 automation 非交互来源的 runtime 派发、事件观测与 sink 绑定。
// POS: automation 执行计划进入 DM/Room runtime 的可信来源分界。
package automation

import (
	"context"
	"errors"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func roomEventObserverForSink(sink *automationexec.ExecutionSink) roomrealtime.RoomEventObserver {
	if sink == nil {
		return nil
	}
	return func(ctx context.Context, event protocol.EventMessage) {
		_ = sink.SendEvent(ctx, event)
	}
}

func (s *Service) bindSink(sessionKey string, sink *automationexec.ExecutionSink) func() {
	if s.permission == nil {
		return func() {}
	}
	s.permission.BindSession(sessionKey, sink)
	return func() {
		s.permission.UnbindSession(sessionKey, sink)
	}
}

func automationRunContext(
	job automationdomain.ScheduledTask,
	runID string,
	resumeAttempt *permissionResumeAttempt,
) *protocol.AutomationRunContext {
	binding := protocol.AutomationRunContext{
		JobID:                    strings.TrimSpace(job.JobID),
		RunID:                    strings.TrimSpace(runID),
		JobName:                  strings.TrimSpace(job.Name),
		PermissionPolicyRevision: job.PermissionPolicy.Revision,
	}
	if resumeAttempt != nil {
		binding.ResumeToolName = strings.TrimSpace(resumeAttempt.toolName)
		binding.ResumeResourceScope = strings.TrimSpace(resumeAttempt.resourceScope)
	}
	if !binding.Valid() {
		return nil
	}
	return &binding
}

func (s *Service) dispatchToSession(ctx context.Context, sessionKey string, roundID string, agentID string, instruction string) error {
	return s.dispatchJobToSession(ctx, automationdomain.ScheduledTask{
		AgentID:     agentID,
		Instruction: instruction,
	}, "", sessionKey, roundID, nil, nil)
}

func (s *Service) dispatchJobToSession(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
	sessionKey string,
	roundID string,
	eventObserver roomrealtime.RoomEventObserver,
	resumeAttempt *permissionResumeAttempt,
) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	jobCtx := contextForJobOwner(ctx, job)
	permissionHandler := s.scheduledTaskPermissionHandler(jobCtx, scheduledPermissionScope{
		Job:           job,
		RunID:         strings.TrimSpace(runID),
		SessionKey:    strings.TrimSpace(sessionKey),
		RoundID:       strings.TrimSpace(roundID),
		ResumeAttempt: resumeAttempt,
	})
	permissionMode := runtimepermission.NormalizeMode(sdkpermission.Mode(
		automationdomain.NormalizePermissionMode(job.PermissionMode),
	))
	runtimeToolPolicy := taskRuntimeToolPolicy(job)
	runContext := automationRunContext(job, runID, resumeAttempt)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(jobCtx, roomrealtime.ChatRequest{
			SessionKey:        sessionKey,
			ConversationID:    parsed.ConversationID,
			Content:           job.Instruction,
			ExecutionOrigin:   "automation",
			TargetAgentIDs:    []string{strings.TrimSpace(job.AgentID)},
			RoundID:           roundID,
			PermissionMode:    permissionMode,
			PermissionHandler: permissionHandler,
			RuntimeToolPolicy: runtimeToolPolicy,
			AutomationRun:     runContext,
			EventObserver:     eventObserver,
		})
	}
	if s.dm == nil {
		return errors.New("automation dm runner is not configured")
	}
	return s.dm.HandleChat(jobCtx, dmsvc.Request{
		SessionKey:        sessionKey,
		AgentID:           firstNonEmpty(job.AgentID, parsed.AgentID),
		Content:           job.Instruction,
		ExecutionOrigin:   "automation",
		RoundID:           roundID,
		PermissionMode:    permissionMode,
		PermissionHandler: permissionHandler,
		RuntimeToolPolicy: runtimeToolPolicy,
		AutomationRun:     runContext,
	})
}

func (s *Service) enqueueMainSessionEvent(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	runID string,
	triggerKind string,
	permissionRequestID string,
	permissionRequestPolicyRevision int,
) (string, error) {
	eventID := s.idFactory("evt")
	if err := s.repository.InsertSystemEvent(
		ctx,
		eventID,
		"scheduled_task.trigger",
		"scheduled_task",
		job.AgentID,
		map[string]any{
			"agent_id":                           job.AgentID,
			"job_id":                             job.JobID,
			"run_id":                             strings.TrimSpace(runID),
			"owner_user_id":                      strings.TrimSpace(job.OwnerUserID),
			"policy_revision":                    job.PermissionPolicy.Revision,
			"permission_request_id":              strings.TrimSpace(permissionRequestID),
			"permission_request_policy_revision": permissionRequestPolicyRevision,
			"text":                               strings.TrimSpace(job.Instruction),
			"trigger_kind":                       triggerKind,
			"session_target_kind":                job.SessionTarget.Kind,
		},
	); err != nil {
		return "", err
	}
	return eventID, nil
}
