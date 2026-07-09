package tool

import (
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type currentTaskContext struct {
	sessionKey string
	channel    string
	ref        string
	threadID   string
	external   bool
}

var currentConversationQueryTerms = []string{
	"这个飞书群", "当前飞书群", "本飞书群",
	"这个群", "本群", "当前群", "群里",
	"这个频道", "当前频道", "本频道",
	"这里", "当前会话", "这个会话", "本会话", "当前对话", "这个对话", "本对话",
	"这个定时任务", "当前定时任务", "本定时任务", "这个任务", "当前任务", "本任务",
	"this group", "current group", "this channel", "current channel", "here",
	"this task", "current task", "this scheduled task", "current scheduled task",
}

func bestMatchingCronJobsForToolQuery(
	jobs []automationdomain.CronJob,
	query string,
	sctx contract.ServerContext,
) []automationdomain.CronJob {
	matches, hasCurrent := bestMatchingCurrentCronJobsForToolQuery(jobs, query, sctx)
	if hasCurrent {
		if queryMentionsCurrentConversation(query) || len(matches) > 0 {
			return matches
		}
	}
	return automationexec.BestMatchingCronJobs(jobs, query)
}

func cronJobMatchesToolQuery(job automationdomain.CronJob, query string, sctx contract.ServerContext) bool {
	if !queryMentionsCurrentConversation(query) {
		return automationexec.CronJobMatchesQuery(job, query)
	}
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return automationexec.CronJobMatchesQuery(job, query)
	}
	if !cronJobMatchesCurrentContext(job, current) {
		return false
	}
	remainder := stripCurrentConversationTerms(query)
	if strings.TrimSpace(remainder) == "" {
		return true
	}
	return automationexec.CronJobMatchesQuery(job, remainder)
}

func filterCronJobsByToolQuery(
	jobs []automationdomain.CronJob,
	query string,
	sctx contract.ServerContext,
) []automationdomain.CronJob {
	currentMatches, hasCurrent := currentCronJobsForToolQuery(jobs, query, sctx)
	if hasCurrent {
		if queryMentionsCurrentConversation(query) || len(currentMatches) > 0 {
			return currentMatches
		}
	}
	return filterCronJobsByPlainQuery(jobs, query)
}

func currentCronJobsForToolQuery(
	jobs []automationdomain.CronJob,
	query string,
	sctx contract.ServerContext,
) ([]automationdomain.CronJob, bool) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return nil, false
	}
	scoped := filterCronJobsByCurrentContext(jobs, current)
	if !queryMentionsCurrentConversation(query) {
		return filterCronJobsByPlainQuery(scoped, query), true
	}
	remainder := stripCurrentConversationTerms(query)
	if strings.TrimSpace(remainder) == "" {
		return scoped, true
	}
	return filterCronJobsByPlainQuery(scoped, remainder), true
}

func bestMatchingCurrentCronJobsForToolQuery(
	jobs []automationdomain.CronJob,
	query string,
	sctx contract.ServerContext,
) ([]automationdomain.CronJob, bool) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return nil, false
	}
	scoped := filterCronJobsByCurrentContext(jobs, current)
	if !queryMentionsCurrentConversation(query) {
		return automationexec.BestMatchingCronJobs(scoped, query), true
	}
	remainder := stripCurrentConversationTerms(query)
	if strings.TrimSpace(remainder) == "" {
		return scoped, true
	}
	return automationexec.BestMatchingCronJobs(scoped, remainder), true
}

func filterCronJobsByPlainQuery(jobs []automationdomain.CronJob, query string) []automationdomain.CronJob {
	matches := make([]automationdomain.CronJob, 0, len(jobs))
	for _, job := range jobs {
		if automationexec.CronJobMatchesQuery(job, query) {
			matches = append(matches, job)
		}
	}
	return matches
}

func currentExternalTaskContextFromServerContext(sctx contract.ServerContext) (currentTaskContext, bool) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok || !current.external {
		return currentTaskContext{}, false
	}
	return current, true
}

func currentTaskContextFromServerContext(sctx contract.ServerContext) (currentTaskContext, bool) {
	sessionKey := strings.TrimSpace(sctx.CurrentSessionKey)
	if sessionKey == "" {
		return currentTaskContext{}, false
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return currentTaskContext{sessionKey: sessionKey}, true
	}
	current := currentTaskContext{
		sessionKey: sessionKey,
		channel:    protocol.NormalizeStoredChannelType(parsed.Channel),
		ref:        strings.TrimSpace(parsed.Ref),
		threadID:   strings.TrimSpace(parsed.ThreadID),
	}
	if parsed.Kind == protocol.SessionKeyKindAgent {
		switch current.channel {
		case protocol.SessionChannelDiscord, protocol.SessionChannelTelegram, protocol.SessionChannelDingTalk, protocol.SessionChannelWeChat, protocol.SessionChannelFeishu:
			current.external = current.ref != ""
		}
	}
	return current, true
}

func filterCronJobsByCurrentExternalContext(
	jobs []automationdomain.CronJob,
	current currentTaskContext,
) []automationdomain.CronJob {
	if !current.external {
		return nil
	}
	return filterCronJobsByCurrentContext(jobs, current)
}

func filterCronJobsByCurrentContext(
	jobs []automationdomain.CronJob,
	current currentTaskContext,
) []automationdomain.CronJob {
	matches := make([]automationdomain.CronJob, 0, len(jobs))
	for _, job := range jobs {
		if cronJobMatchesCurrentContext(job, current) {
			matches = append(matches, job)
		}
	}
	return matches
}

func cronJobMatchesCurrentContext(job automationdomain.CronJob, current currentTaskContext) bool {
	if strings.TrimSpace(current.sessionKey) == "" {
		return false
	}
	if strings.TrimSpace(job.Source.SessionKey) == current.sessionKey {
		return true
	}
	if strings.TrimSpace(job.SessionTarget.BoundSessionKey) == current.sessionKey {
		return true
	}
	return deliveryTargetMatchesCurrentContext(job.Delivery, current)
}

func taskEventMatchesCurrentContext(event automationdomain.CronTaskEvent, current currentTaskContext) bool {
	if strings.TrimSpace(current.sessionKey) == "" {
		return false
	}
	if eventDetailString(event.Detail, "source_session_key") == current.sessionKey {
		return true
	}
	if eventDetailString(event.Detail, "bound_session_key") == current.sessionKey {
		return true
	}
	return deliveryTargetMatchesCurrentContext(automationdomain.DeliveryTarget{
		Channel:   eventDetailString(event.Detail, "delivery_channel"),
		To:        eventDetailString(event.Detail, "delivery_to"),
		AccountID: eventDetailString(event.Detail, "delivery_account_id"),
		ThreadID:  eventDetailString(event.Detail, "delivery_thread_id"),
	}, current)
}

func taskEventMatchesCurrentExternalContext(event automationdomain.CronTaskEvent, current currentTaskContext) bool {
	if !current.external {
		return false
	}
	return taskEventMatchesCurrentContext(event, current)
}

func deliveryTargetMatchesCurrentContext(target automationdomain.DeliveryTarget, current currentTaskContext) bool {
	to := strings.TrimSpace(target.To)
	if to == "" {
		return false
	}
	if to == current.sessionKey {
		return true
	}
	if !current.external {
		return false
	}
	if protocol.NormalizeStoredChannelType(target.Channel) != current.channel {
		return false
	}
	if to == current.ref || to == current.sessionKey {
		return true
	}
	accountID := strings.TrimSpace(target.AccountID)
	if accountID != "" && accountID+":"+to == current.ref {
		return true
	}
	if strings.Contains(current.ref, ":") && strings.HasSuffix(current.ref, ":"+to) {
		return true
	}
	return false
}

func queryMentionsCurrentConversation(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	for _, term := range currentConversationQueryTerms {
		if strings.Contains(normalized, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func stripCurrentConversationTerms(query string) string {
	remainder := strings.ToLower(strings.TrimSpace(query))
	for _, term := range currentConversationQueryTerms {
		remainder = strings.ReplaceAll(remainder, strings.ToLower(term), " ")
	}
	return strings.Join(strings.Fields(remainder), " ")
}

func eventDetailString(detail map[string]any, key string) string {
	if detail == nil {
		return ""
	}
	value, ok := detail[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
