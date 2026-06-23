package automation

import (
	"context"
	"errors"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func roomEventObserverForSink(sink *automationdomain.ExecutionSink) roomsvc.RoomEventObserver {
	if sink == nil {
		return nil
	}
	return func(ctx context.Context, event protocol.EventMessage) {
		_ = sink.SendEvent(ctx, event)
	}
}

func (s *Service) bindSink(sessionKey string, sink *automationdomain.ExecutionSink) func() {
	if s.permission == nil {
		return func() {}
	}
	s.permission.BindSession(sessionKey, sink)
	return func() {
		s.permission.UnbindSession(sessionKey, sink)
	}
}

func buildCronInstruction(job protocol.CronJob) string {
	marker := buildCronMarker(job)
	instruction := strings.TrimSpace(job.Instruction)
	if instruction == "" {
		return marker
	}
	return marker + " " + instruction
}

func buildCronMarker(job protocol.CronJob) string {
	jobID := strings.TrimSpace(job.JobID)
	if jobID == "" {
		jobID = "unknown"
	}
	name := normalizeCronMarkerLabel(job.Name)
	if name == "" {
		return "[cron:" + jobID + "]"
	}
	return "[cron:" + jobID + " " + name + "]"
}

func normalizeCronMarkerLabel(value string) string {
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
	return s.dispatchJobToSession(ctx, protocol.CronJob{
		AgentID:     agentID,
		Instruction: instruction,
	}, sessionKey, roundID, nil)
}

func (s *Service) dispatchJobToSession(
	ctx context.Context,
	job protocol.CronJob,
	sessionKey string,
	roundID string,
	eventObserver roomsvc.RoomEventObserver,
) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	jobCtx := contextForJobOwner(ctx, job)
	permissionHandler := s.scheduledTaskPermissionHandler(jobCtx, job)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		if s.room == nil {
			return errors.New("shared room session automation 暂不支持")
		}
		return s.room.HandleChat(jobCtx, roomsvc.ChatRequest{
			SessionKey:        sessionKey,
			ConversationID:    parsed.ConversationID,
			Content:           job.Instruction,
			TargetAgentIDs:    []string{strings.TrimSpace(job.AgentID)},
			RoundID:           roundID,
			ReqID:             roundID,
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
		ReqID:             roundID,
		PermissionMode:    sdkpermission.ModeDefault,
		PermissionHandler: permissionHandler,
	})
}

func (s *Service) enqueueMainSessionEvent(ctx context.Context, job protocol.CronJob, triggerKind string) (string, error) {
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
			"text":                buildCronInstruction(job),
			"trigger_kind":        triggerKind,
			"session_target_kind": job.SessionTarget.Kind,
		},
	); err != nil {
		return "", err
	}
	return eventID, nil
}
