package tool

import (
	"context"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

func currentConversationDailyReport(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
) (*automationdomain.ScheduledTaskDailyReport, bool, error) {
	if !shouldUseCurrentConversationDailyReport(sctx, args) {
		return nil, false, nil
	}
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return nil, false, nil
	}
	agentID, err := resolveListAgentID(sctx, "")
	if err != nil {
		return nil, true, err
	}
	scopedCtx := scopedToolContext(ctx, sctx)
	jobs, err := svc.ListTasks(scopedCtx, agentID)
	if err != nil {
		return nil, true, err
	}
	matches := filterScheduledTasksByCurrentContext(jobs, current)
	if len(matches) == 0 {
		payload, emptyErr := emptyCurrentConversationDailyReport(scopedCtx, svc, sctx, args, agentID)
		return payload, true, emptyErr
	}
	payload, err := buildCurrentConversationDailyReport(scopedCtx, svc, sctx, args, matches)
	if err != nil {
		return nil, true, err
	}
	return payload, true, nil
}

func emptyCurrentConversationDailyReport(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
	agentID string,
) (*automationdomain.ScheduledTaskDailyReport, error) {
	report, err := svc.GetDailyReport(ctx, automationdomain.ScheduledTaskDailyReportInput{
		Date:     argx.String(args, "date"),
		Timezone: argx.FirstNonEmpty(argx.String(args, "timezone"), sctx.DefaultTimezone),
		AgentID:  agentID,
	})
	if err != nil {
		return nil, err
	}
	if report == nil {
		return &automationdomain.ScheduledTaskDailyReport{
			Timezone: argx.FirstNonEmpty(argx.String(args, "timezone"), sctx.DefaultTimezone, "Asia/Shanghai"),
			AgentID:  agentID,
			Tasks:    []automationdomain.ScheduledTaskDailyReportItem{},
		}, nil
	}
	report.JobID = ""
	report.Totals = automationdomain.ScheduledTaskDailyReportTotals{}
	report.Tasks = []automationdomain.ScheduledTaskDailyReportItem{}
	return report, nil
}

func shouldUseCurrentConversationDailyReport(sctx contract.ServerContext, args map[string]any) bool {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return false
	}
	if strings.TrimSpace(argx.String(args, "job_id")) != "" ||
		strings.TrimSpace(argx.String(args, "agent_id")) != "" {
		return false
	}
	query := strings.TrimSpace(argx.String(args, "query"))
	if query == "" {
		return current.external
	}
	if !queryMentionsCurrentConversation(query) {
		return false
	}
	return isGenericDailyReportRemainder(stripCurrentConversationTerms(query))
}

func isGenericDailyReportRemainder(remainder string) bool {
	normalized := strings.ToLower(strings.TrimSpace(remainder))
	if normalized == "" {
		return true
	}
	for _, term := range genericDailyReportQueryTerms {
		normalized = strings.ReplaceAll(normalized, strings.ToLower(term), " ")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(normalized), " ")) == ""
}

var genericDailyReportQueryTerms = []string{
	"的", "了", "一下",
	"定时任务", "任务", "自动任务",
	"发送情况", "投递情况", "运行情况", "发送状态", "投递状态", "运行状态",
	"发送", "投递", "运行", "情况", "状态",
	"今天", "今日",
	"the", "a", "an",
	"scheduled tasks", "scheduled task", "tasks", "task", "automation", "automations",
	"delivery status", "run status", "delivery", "runs", "run", "status",
	"today", "daily report", "report",
}

func buildCurrentConversationDailyReport(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
	jobs []automationdomain.ScheduledTask,
) (*automationdomain.ScheduledTaskDailyReport, error) {
	result := &automationdomain.ScheduledTaskDailyReport{
		Timezone: argx.FirstNonEmpty(argx.String(args, "timezone"), sctx.DefaultTimezone),
		Tasks:    []automationdomain.ScheduledTaskDailyReportItem{},
	}
	for _, job := range jobs {
		report, err := svc.GetDailyReport(ctx, automationdomain.ScheduledTaskDailyReportInput{
			Date:     argx.String(args, "date"),
			Timezone: result.Timezone,
			JobID:    job.JobID,
		})
		if err != nil {
			return nil, err
		}
		if report == nil {
			continue
		}
		mergeCurrentConversationDailyReport(result, report)
	}
	if result.Timezone == "" {
		result.Timezone = "Asia/Shanghai"
	}
	return result, nil
}

func mergeCurrentConversationDailyReport(target *automationdomain.ScheduledTaskDailyReport, source *automationdomain.ScheduledTaskDailyReport) {
	if target.Date == "" {
		target.Date = source.Date
	}
	if target.Timezone == "" {
		target.Timezone = source.Timezone
	}
	if target.AgentID == "" {
		target.AgentID = source.AgentID
	} else if source.AgentID != "" && target.AgentID != source.AgentID {
		target.AgentID = ""
	}
	if target.StartAt.IsZero() {
		target.StartAt = source.StartAt
	}
	if target.EndAt.IsZero() {
		target.EndAt = source.EndAt
	}
	target.Tasks = append(target.Tasks, source.Tasks...)
	target.Totals.TaskCount += source.Totals.TaskCount
	target.Totals.EnabledTaskCount += source.Totals.EnabledTaskCount
	target.Totals.RunningTaskCount += source.Totals.RunningTaskCount
	target.Totals.RunCount += source.Totals.RunCount
	target.Totals.SucceededRunCount += source.Totals.SucceededRunCount
	target.Totals.FailedRunCount += source.Totals.FailedRunCount
	target.Totals.CancelledRunCount += source.Totals.CancelledRunCount
	target.Totals.SkippedRunCount += source.Totals.SkippedRunCount
	target.Totals.DeliveredRunCount += source.Totals.DeliveredRunCount
	target.Totals.DeliveryFailedRunCount += source.Totals.DeliveryFailedRunCount
	target.Totals.DeliveryPendingRunCount += source.Totals.DeliveryPendingRunCount
	target.Totals.DeliverySkippedRunCount += source.Totals.DeliverySkippedRunCount
	target.Totals.DeliveryDeadLetterRunCount += source.Totals.DeliveryDeadLetterRunCount
	target.Totals.DeliveryNotNeededCount += source.Totals.DeliveryNotNeededCount
	target.Totals.DeliveryNotAttemptedCount += source.Totals.DeliveryNotAttemptedCount
}
