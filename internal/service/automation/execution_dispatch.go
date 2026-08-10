package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func buildScheduledTaskInstruction(job automationdomain.ScheduledTask) string {
	marker := buildScheduledTaskMarker(job)
	instruction := strings.TrimSpace(job.Instruction)
	if instruction == "" {
		return marker
	}
	return marker + " " + instruction
}

func buildPermissionResumeInstruction(
	job automationdomain.ScheduledTask,
	request *automationdomain.AutomationPermissionRequest,
) string {
	instruction := buildScheduledTaskInstruction(job)
	if request == nil {
		return instruction
	}
	toolName := normalizePermissionResumeLabel(request.Capability.ToolName)
	if toolName == "" {
		return instruction
	}
	target := normalizePermissionResumeLabel(request.Capability.ResourceScope)
	targetCopy := ""
	if target != "" {
		targetCopy = fmt.Sprintf("，目标仍为 `%s`", target)
	}
	return fmt.Sprintf(
		"%s\n\n[权限续跑] 用户已经处理上一轮的权限请求。请忽略上一轮的拒绝结论，并在本轮重新调用工具 `%s`%s，沿用原任务指定的参数。不得仅引用上一轮失败直接结束；收到这次工具调用的实际结果后再总结本次运行。",
		instruction,
		toolName,
		targetCopy,
	)
}

func normalizePermissionResumeLabel(value string) string {
	cleaned := strings.NewReplacer(
		"`", "",
		"\r", " ",
		"\n", " ",
		"\t", " ",
	).Replace(strings.TrimSpace(value))
	return strings.Join(strings.Fields(cleaned), " ")
}

func buildScheduledTaskMarker(job automationdomain.ScheduledTask) string {
	jobID := strings.TrimSpace(job.JobID)
	if jobID == "" {
		jobID = "unknown"
	}
	name := normalizeScheduledTaskMarkerLabel(job.Name)
	if name == "" {
		return "[scheduled-task:" + jobID + "]"
	}
	return "[scheduled-task:" + jobID + " " + name + "]"
}

func normalizeScheduledTaskMarkerLabel(value string) string {
	cleaned := strings.NewReplacer(
		"[", " ",
		"]", " ",
		"\r", " ",
		"\n", " ",
		"\t", " ",
	).Replace(strings.TrimSpace(value))
	return strings.Join(strings.Fields(cleaned), " ")
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
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(jobCtx, roomrealtime.ChatRequest{
			SessionKey:        sessionKey,
			ConversationID:    parsed.ConversationID,
			Content:           job.Instruction,
			TargetAgentIDs:    []string{strings.TrimSpace(job.AgentID)},
			RoundID:           roundID,
			PermissionMode:    sdkpermission.ModeDefault,
			PermissionHandler: permissionHandler,
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
		RoundID:           roundID,
		PermissionMode:    sdkpermission.ModeDefault,
		PermissionHandler: permissionHandler,
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
			"text":                               buildScheduledTaskInstruction(job),
			"trigger_kind":                       triggerKind,
			"session_target_kind":                job.SessionTarget.Kind,
		},
	); err != nil {
		return "", err
	}
	return eventID, nil
}
