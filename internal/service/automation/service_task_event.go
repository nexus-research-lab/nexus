package automation

import (
	"context"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func (s *Service) ListTaskEvents(ctx context.Context, jobID string, limit int) ([]automationdomain.CronTaskEvent, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, automationdomain.ErrJobNotFound
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	job, err := s.repository.GetCronJob(ctx, ownerUserID, jobID)
	if err != nil {
		return nil, err
	}
	events, err := s.repository.ListTaskEventsByJob(ctx, ownerUserID, jobID, limit)
	if err != nil {
		return nil, err
	}
	if job == nil && len(events) == 0 {
		return nil, automationdomain.ErrJobNotFound
	}
	return events, nil
}

func (s *Service) recordTaskEvent(ctx context.Context, action string, job automationdomain.CronJob, runID string, detail map[string]any) {
	jobID := strings.TrimSpace(job.JobID)
	action = strings.TrimSpace(action)
	if jobID == "" || action == "" {
		return
	}
	if detail == nil {
		detail = map[string]any{}
	}
	actorUserID := authctx.OwnerUserID(ctx)
	actorAgentID := strings.TrimSpace(job.Source.CreatorAgentID)
	if contextActorAgentID, ok := automationexec.ActorAgentID(ctx); ok {
		actorAgentID = contextActorAgentID
	}
	event := automationdomain.CronTaskEvent{
		EventID:      s.idFactory("task_evt"),
		JobID:        jobID,
		OwnerUserID:  job.OwnerUserID,
		AgentID:      job.AgentID,
		Action:       action,
		ActorUserID:  actorUserID,
		ActorAgentID: actorAgentID,
		RunID:        runID,
		Detail:       detail,
		CreatedAt:    s.nowFn(),
	}
	if err := s.repository.InsertTaskEvent(ctx, automationstore.TaskEventInput{
		EventID:      event.EventID,
		JobID:        event.JobID,
		OwnerUserID:  event.OwnerUserID,
		AgentID:      event.AgentID,
		Action:       event.Action,
		ActorUserID:  event.ActorUserID,
		ActorAgentID: event.ActorAgentID,
		RunID:        event.RunID,
		Detail:       event.Detail,
	}); err != nil {
		s.loggerFor(ctx).Warn("写入定时任务管理审计失败",
			"job_id", job.JobID,
			"action", action,
			"err", err,
		)
		return
	}
	if s.taskNotifier != nil {
		s.taskNotifier.NotifyTaskEvent(ctx, event)
	}
}

func updateTaskEventAction(input automationdomain.UpdateJobInput, next automationdomain.CronJob) string {
	if input.Enabled != nil && onlyEnabledChanged(input) {
		if next.Enabled {
			return automationdomain.TaskEventActionEnable
		}
		return automationdomain.TaskEventActionDisable
	}
	return automationdomain.TaskEventActionUpdate
}

func updateTaskEventDetail(input automationdomain.UpdateJobInput, before automationdomain.CronJob, after automationdomain.CronJob) map[string]any {
	fields := changedTaskFields(input)
	detail := taskEventJobSnapshot(after)
	detail["changed_fields"] = fields
	if input.Enabled != nil {
		detail["enabled"] = after.Enabled
		detail["previous_enabled"] = before.Enabled
		if strings.TrimSpace(before.RunningRunID) != "" {
			detail["active_run_id"] = strings.TrimSpace(before.RunningRunID)
		}
	}
	return detail
}

func updateTaskEventRunID(input automationdomain.UpdateJobInput, before automationdomain.CronJob) string {
	if input.Enabled == nil || *input.Enabled {
		return ""
	}
	return strings.TrimSpace(before.RunningRunID)
}

func deleteTaskEventDetail(job automationdomain.CronJob, cancelledRunID string, cancelledRun bool, deadLetteredDeliveryRunIDs []string) map[string]any {
	detail := taskEventJobSnapshot(job)
	if strings.TrimSpace(cancelledRunID) != "" {
		detail["cancelled_run_id"] = strings.TrimSpace(cancelledRunID)
		detail["cancelled_active_run"] = cancelledRun
	}
	if len(deadLetteredDeliveryRunIDs) > 0 {
		detail["dead_lettered_delivery_run_ids"] = deadLetteredDeliveryRunIDs
	}
	return detail
}

func deliveryRetryTaskEventDetail(run automationdomain.CronRun) map[string]any {
	detail := map[string]any{
		"delivery_status":   strings.TrimSpace(run.DeliveryStatus),
		"delivery_attempts": run.DeliveryAttempts,
		"delivery_mode":     strings.TrimSpace(run.DeliveryMode),
		"delivery_to":       strings.TrimSpace(run.DeliveryTo),
	}
	if run.DeliveryError != nil && strings.TrimSpace(*run.DeliveryError) != "" {
		detail["delivery_error"] = strings.TrimSpace(*run.DeliveryError)
	}
	if run.DeliveryNextAttemptAt != nil {
		detail["delivery_next_attempt_at"] = *run.DeliveryNextAttemptAt
	}
	if run.DeliveryDeadLetterAt != nil {
		detail["delivery_dead_letter_at"] = *run.DeliveryDeadLetterAt
	}
	if run.DeliveredAt != nil {
		detail["delivered_at"] = *run.DeliveredAt
	}
	return detail
}

func taskEventJobSnapshot(job automationdomain.CronJob) map[string]any {
	detail := map[string]any{
		"name":                 job.Name,
		"instruction":          job.Instruction,
		"enabled":              job.Enabled,
		"schedule_kind":        job.Schedule.Kind,
		"schedule_timezone":    job.Schedule.Timezone,
		"execution_kind":       automationdomain.NormalizeExecutionKind(job.ExecutionKind),
		"session_target_kind":  job.SessionTarget.Kind,
		"delivery_mode":        job.Delivery.Mode,
		"delivery_channel":     job.Delivery.Channel,
		"delivery_to":          job.Delivery.To,
		"delivery_account_id":  job.Delivery.AccountID,
		"delivery_thread_id":   job.Delivery.ThreadID,
		"source_kind":          job.Source.Kind,
		"source_context_type":  job.Source.ContextType,
		"source_context_id":    job.Source.ContextID,
		"source_context_label": job.Source.ContextLabel,
		"source_session_key":   job.Source.SessionKey,
		"source_session_label": job.Source.SessionLabel,
		"overlap_policy":       automationdomain.NormalizeOverlapPolicy(job.OverlapPolicy),
	}
	if job.Schedule.RunAt != nil {
		detail["schedule_run_at"] = strings.TrimSpace(*job.Schedule.RunAt)
	}
	if job.Schedule.IntervalSeconds != nil {
		detail["schedule_interval_seconds"] = *job.Schedule.IntervalSeconds
	}
	if job.Schedule.CronExpression != nil {
		detail["schedule_cron_expression"] = strings.TrimSpace(*job.Schedule.CronExpression)
	}
	if strings.TrimSpace(job.SessionTarget.BoundSessionKey) != "" {
		detail["bound_session_key"] = strings.TrimSpace(job.SessionTarget.BoundSessionKey)
	}
	if strings.TrimSpace(job.SessionTarget.NamedSessionKey) != "" {
		detail["named_session_key"] = strings.TrimSpace(job.SessionTarget.NamedSessionKey)
	}
	return detail
}

func changedTaskFields(input automationdomain.UpdateJobInput) []string {
	fields := []string{}
	if input.Name != nil {
		fields = append(fields, "name")
	}
	if input.Schedule != nil {
		fields = append(fields, "schedule")
	}
	if input.Instruction != nil {
		fields = append(fields, "instruction")
	}
	if input.ExecutionKind != nil {
		fields = append(fields, "execution_kind")
	}
	if input.SessionTarget != nil {
		fields = append(fields, "session_target")
	}
	if input.Delivery != nil {
		fields = append(fields, "delivery")
	}
	if input.Source != nil {
		fields = append(fields, "source")
	}
	if input.OverlapPolicy != nil {
		fields = append(fields, "overlap_policy")
	}
	if input.Enabled != nil {
		fields = append(fields, "enabled")
	}
	return fields
}

func onlyEnabledChanged(input automationdomain.UpdateJobInput) bool {
	return input.Name == nil &&
		input.Schedule == nil &&
		input.Instruction == nil &&
		input.ExecutionKind == nil &&
		input.SessionTarget == nil &&
		input.Delivery == nil &&
		input.Source == nil &&
		input.OverlapPolicy == nil &&
		input.Enabled != nil
}
