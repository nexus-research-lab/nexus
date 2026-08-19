// INPUT: runtime Actor 的结构化 SessionKey、任务来源/执行/投递快照与自然语言查询。
// OUTPUT: 只属于当前 DM/Room/外部 IM 会话的任务集合和日报聚合。
// POS: Automation CLI 的会话语义边界；“这里/当前会话”与外部 IM 默认查询不能放大到 Agent 全域。
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeCurrentTaskContext struct {
	sessionKey string
	channel    string
	accountID  string
	ref        string
	threadID   string
	external   bool
}

var runtimeCurrentConversationQueryTerms = []string{
	"这个飞书群", "当前飞书群", "本飞书群", "这个群", "本群", "当前群", "群里",
	"这个频道", "当前频道", "本频道", "这里", "当前会话", "这个会话", "本会话",
	"当前对话", "这个对话", "本对话", "这个定时任务", "当前定时任务", "本定时任务",
	"这个任务", "当前任务", "本任务", "this group", "current group", "this channel",
	"current channel", "here", "this task", "current task", "this scheduled task",
	"current scheduled task",
}

func runtimeTaskContextFromActor(actor RuntimeCommandActor) (runtimeCurrentTaskContext, bool) {
	sessionKey := strings.TrimSpace(actor.SessionKey)
	if sessionKey == "" {
		return runtimeCurrentTaskContext{}, false
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return runtimeCurrentTaskContext{sessionKey: sessionKey}, true
	}
	current := runtimeCurrentTaskContext{
		sessionKey: sessionKey,
		channel:    protocol.NormalizeStoredChannelType(parsed.Channel),
		accountID:  strings.TrimSpace(parsed.AccountID),
		ref:        strings.TrimSpace(parsed.Ref),
		threadID:   strings.TrimSpace(parsed.ThreadID),
	}
	if parsed.Kind == protocol.SessionKeyKindAgent &&
		current.channel != protocol.SessionChannelWebSocket &&
		current.channel != protocol.SessionChannelInternalSegment &&
		current.channel != "automation" {
		current.external = current.ref != ""
	}
	return current, true
}

func runtimeBestMatchingTasks(
	jobs []automationdomain.ScheduledTask,
	query string,
	actor RuntimeCommandActor,
) []automationdomain.ScheduledTask {
	current, ok := runtimeTaskContextFromActor(actor)
	if !ok {
		return automationexec.BestMatchingScheduledTasks(jobs, query)
	}
	scoped := runtimeFilterTasksByCurrentContext(jobs, current)
	remainder := query
	mentionsCurrent := runtimeQueryMentionsCurrentConversation(query)
	if mentionsCurrent {
		remainder = runtimeStripCurrentConversationTerms(query)
	}
	matches := automationexec.BestMatchingScheduledTasks(scoped, remainder)
	if current.external || mentionsCurrent || len(matches) > 0 {
		return matches
	}
	return automationexec.BestMatchingScheduledTasks(jobs, query)
}

func runtimeFilterTasksForList(
	jobs []automationdomain.ScheduledTask,
	query string,
	actor RuntimeCommandActor,
) []automationdomain.ScheduledTask {
	current, ok := runtimeTaskContextFromActor(actor)
	if !ok {
		return runtimeFilterTasksByPlainQuery(jobs, query)
	}
	scoped := runtimeFilterTasksByCurrentContext(jobs, current)
	remainder := query
	mentionsCurrent := runtimeQueryMentionsCurrentConversation(query)
	if mentionsCurrent {
		remainder = runtimeStripCurrentConversationTerms(query)
	}
	matches := runtimeFilterTasksByPlainQuery(scoped, remainder)
	if current.external || mentionsCurrent || len(matches) > 0 {
		return matches
	}
	return runtimeFilterTasksByPlainQuery(jobs, query)
}

func runtimeFilterTasksByPlainQuery(
	jobs []automationdomain.ScheduledTask,
	query string,
) []automationdomain.ScheduledTask {
	matches := make([]automationdomain.ScheduledTask, 0, len(jobs))
	for _, job := range jobs {
		if strings.TrimSpace(query) == "" || automationexec.ScheduledTaskMatchesQuery(job, query) {
			matches = append(matches, job)
		}
	}
	return matches
}

func runtimeFilterTasksByCurrentContext(
	jobs []automationdomain.ScheduledTask,
	current runtimeCurrentTaskContext,
) []automationdomain.ScheduledTask {
	matches := make([]automationdomain.ScheduledTask, 0, len(jobs))
	for _, job := range jobs {
		if runtimeTaskMatchesCurrentContext(job, current) {
			matches = append(matches, job)
		}
	}
	return matches
}

func runtimeTaskMatchesCurrentContext(
	job automationdomain.ScheduledTask,
	current runtimeCurrentTaskContext,
) bool {
	if strings.TrimSpace(current.sessionKey) == "" {
		return false
	}
	if strings.TrimSpace(job.Source.SessionKey) == current.sessionKey ||
		strings.TrimSpace(job.SessionTarget.BoundSessionKey) == current.sessionKey ||
		strings.TrimSpace(job.Delivery.SessionKey) == current.sessionKey {
		return true
	}
	return runtimeDeliveryMatchesCurrentContext(job.Delivery, current)
}

func runtimeTaskEventMatchesCurrentContext(
	event automationdomain.ScheduledTaskEvent,
	current runtimeCurrentTaskContext,
) bool {
	if runtimeEventDetailString(event.Detail, "source_session_key") == current.sessionKey ||
		runtimeEventDetailString(event.Detail, "bound_session_key") == current.sessionKey ||
		runtimeEventDetailString(event.Detail, "delivery_session_key") == current.sessionKey {
		return true
	}
	return runtimeDeliveryMatchesCurrentContext(automationdomain.DeliveryTarget{
		Channel:   runtimeEventDetailString(event.Detail, "delivery_channel"),
		To:        runtimeEventDetailString(event.Detail, "delivery_to"),
		AccountID: runtimeEventDetailString(event.Detail, "delivery_account_id"),
		ThreadID:  runtimeEventDetailString(event.Detail, "delivery_thread_id"),
	}, current)
}

func runtimeDeliveryMatchesCurrentContext(
	target automationdomain.DeliveryTarget,
	current runtimeCurrentTaskContext,
) bool {
	to := strings.TrimSpace(target.To)
	if to == "" {
		return false
	}
	if to == current.sessionKey {
		return true
	}
	if !current.external || protocol.NormalizeStoredChannelType(target.Channel) != current.channel {
		return false
	}
	targetAccountID := strings.TrimSpace(target.AccountID)
	if current.accountID != "" && targetAccountID != current.accountID {
		return false
	}
	if current.accountID == "" && targetAccountID != "" && targetAccountID+":"+to != current.ref {
		return false
	}
	if threadID := strings.TrimSpace(target.ThreadID); threadID != "" && threadID != current.threadID {
		return false
	}
	return to == current.ref ||
		targetAccountID != "" && targetAccountID+":"+to == current.ref ||
		strings.Contains(current.ref, ":") && strings.HasSuffix(current.ref, ":"+to)
}

func runtimeQueryMentionsCurrentConversation(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	for _, term := range runtimeCurrentConversationQueryTerms {
		if strings.Contains(normalized, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func runtimeStripCurrentConversationTerms(query string) string {
	remainder := strings.ToLower(strings.TrimSpace(query))
	for _, term := range runtimeCurrentConversationQueryTerms {
		remainder = strings.ReplaceAll(remainder, strings.ToLower(term), " ")
	}
	for _, term := range []string{
		"定时任务", "自动任务", "任务", "的", "一下", "列出", "查找", "查看", "看看", "有哪些",
		"scheduled tasks", "scheduled task", "tasks", "task", "show", "list", "find", "get",
	} {
		remainder = strings.ReplaceAll(remainder, strings.ToLower(term), " ")
	}
	return strings.Join(strings.Fields(remainder), " ")
}

func runtimeEventDetailString(detail map[string]any, key string) string {
	if value, ok := detail[key]; ok && value != nil {
		if text, textOK := value.(string); textOK {
			return strings.TrimSpace(text)
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func (s *Service) runtimeCurrentContextHistoryItems(
	ctx context.Context,
	actor RuntimeCommandActor,
	agentID string,
	query string,
	includeActive bool,
	enabled *bool,
	limit int,
) ([]automationdomain.ScheduledTaskHistoryItem, error) {
	current, ok := runtimeTaskContextFromActor(actor)
	if !ok {
		return []automationdomain.ScheduledTaskHistoryItem{}, nil
	}
	jobIDs := make([]string, 0, limit)
	seen := map[string]bool{}
	appendJobID := func(jobID string) {
		jobID = strings.TrimSpace(jobID)
		if jobID != "" && !seen[jobID] {
			seen[jobID] = true
			jobIDs = append(jobIDs, jobID)
		}
	}
	jobs, err := s.ListTasks(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for _, job := range runtimeFilterTasksByPlainQuery(
		runtimeFilterTasksByCurrentContext(jobs, current), query,
	) {
		appendJobID(job.JobID)
	}
	ownerUserID, _ := scopedOwnerUserID(ctx)
	lookupKeys := []string{current.sessionKey}
	if current.external && current.ref != "" && current.ref != current.sessionKey {
		lookupKeys = append(lookupKeys, current.ref)
	}
	seenEvents := map[string]bool{}
	for _, lookupKey := range lookupKeys {
		events, searchErr := s.repository.SearchTaskEvents(ctx, ownerUserID, agentID, lookupKey, 200)
		if searchErr != nil {
			return nil, searchErr
		}
		for _, event := range events {
			if seenEvents[event.EventID] || !runtimeTaskEventMatchesCurrentContext(event, current) ||
				!runtimeTaskEventMatchesQuery(event, query) {
				continue
			}
			seenEvents[event.EventID] = true
			appendJobID(event.JobID)
		}
	}
	items := make([]automationdomain.ScheduledTaskHistoryItem, 0, min(limit, len(jobIDs)))
	for _, jobID := range jobIDs {
		matches, searchErr := s.SearchTaskHistory(ctx, automationdomain.ScheduledTaskHistorySearchInput{
			Query: jobID, AgentID: agentID, IncludeActive: includeActive, IncludeDeleted: true, Limit: 10,
		})
		if searchErr != nil {
			return nil, searchErr
		}
		for _, item := range matches {
			if strings.TrimSpace(item.JobID) != jobID || !includeActive && !item.Deleted {
				continue
			}
			if enabled != nil && (item.Enabled == nil || *item.Enabled != *enabled) {
				continue
			}
			items = append(items, item)
			break
		}
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func runtimeTaskEventMatchesQuery(event automationdomain.ScheduledTaskEvent, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	detail, _ := json.Marshal(event.Detail)
	haystack := strings.ToLower(strings.Join([]string{
		event.JobID, event.Action, event.AgentID, event.RunID, string(detail),
	}, " "))
	for _, variant := range automationexec.QueryVariants(query) {
		if strings.Contains(haystack, strings.ToLower(strings.TrimSpace(variant))) {
			return true
		}
	}
	return false
}

func (s *Service) runtimeCurrentConversationReport(
	ctx context.Context,
	actor RuntimeCommandActor,
	input automationdomain.AutomationCommandInput,
	agentID string,
) (*automationdomain.ScheduledTaskDailyReport, bool, error) {
	current, ok := runtimeTaskContextFromActor(actor)
	if !ok || strings.TrimSpace(input.JobID) != "" || strings.TrimSpace(input.AgentID) != "" {
		return nil, false, nil
	}
	query := strings.TrimSpace(input.Query)
	if query == "" && !current.external {
		return nil, false, nil
	}
	if query != "" && (!runtimeQueryMentionsCurrentConversation(query) ||
		!runtimeGenericReportRemainder(runtimeStripCurrentConversationTerms(query))) {
		return nil, false, nil
	}
	jobs, err := s.ListTasks(ctx, agentID)
	if err != nil {
		return nil, true, err
	}
	jobs = runtimeFilterTasksByCurrentContext(jobs, current)
	timezone := firstNonEmpty(input.Timezone, actor.DefaultTimezone)
	result := &automationdomain.ScheduledTaskDailyReport{
		Timezone: timezone, AgentID: agentID, Tasks: []automationdomain.ScheduledTaskDailyReportItem{},
	}
	if len(jobs) == 0 {
		base, reportErr := s.GetDailyReport(ctx, automationdomain.ScheduledTaskDailyReportInput{
			Date: strings.TrimSpace(input.Date), Timezone: timezone, AgentID: agentID,
		})
		if reportErr != nil {
			return nil, true, reportErr
		}
		if base != nil {
			result.Date, result.StartAt, result.EndAt = base.Date, base.StartAt, base.EndAt
			result.Timezone = base.Timezone
		}
		return result, true, nil
	}
	for _, job := range jobs {
		report, reportErr := s.GetDailyReport(ctx, automationdomain.ScheduledTaskDailyReportInput{
			Date: strings.TrimSpace(input.Date), Timezone: timezone, JobID: job.JobID,
		})
		if reportErr != nil {
			return nil, true, reportErr
		}
		if report != nil {
			runtimeMergeDailyReport(result, report)
		}
	}
	return result, true, nil
}

func runtimeGenericReportRemainder(remainder string) bool {
	normalized := strings.ToLower(strings.TrimSpace(remainder))
	for _, term := range []string{
		"的", "了", "一下", "定时任务", "任务", "自动任务", "发送情况", "投递情况",
		"运行情况", "发送状态", "投递状态", "运行状态", "发送", "投递", "运行", "情况",
		"状态", "今天", "今日", "the", "a", "an", "scheduled tasks", "scheduled task",
		"报告", "日报",
		"tasks", "task", "automation", "automations", "delivery status", "run status",
		"delivery", "runs", "run", "status", "today", "daily report", "report",
	} {
		normalized = strings.ReplaceAll(normalized, strings.ToLower(term), " ")
	}
	return len(strings.Fields(normalized)) == 0
}

func runtimeMergeDailyReport(
	target *automationdomain.ScheduledTaskDailyReport,
	source *automationdomain.ScheduledTaskDailyReport,
) {
	if target.Date == "" {
		target.Date, target.StartAt, target.EndAt = source.Date, source.StartAt, source.EndAt
	}
	if target.Timezone == "" {
		target.Timezone = source.Timezone
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
