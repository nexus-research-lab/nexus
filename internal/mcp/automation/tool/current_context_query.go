package tool

import (
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
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

func bestMatchingScheduledTasksForToolQuery(
	jobs []automationdomain.ScheduledTask,
	query string,
	sctx contract.ServerContext,
) []automationdomain.ScheduledTask {
	matches, hasCurrent := bestMatchingCurrentScheduledTasksForToolQuery(jobs, query, sctx)
	if hasCurrent {
		if queryMentionsCurrentConversation(query) || len(matches) > 0 {
			return matches
		}
	}
	return automationexec.BestMatchingScheduledTasks(jobs, query)
}

func filterScheduledTasksByToolQuery(
	jobs []automationdomain.ScheduledTask,
	query string,
	sctx contract.ServerContext,
) []automationdomain.ScheduledTask {
	currentMatches, hasCurrent := currentScheduledTasksForToolQuery(jobs, query, sctx)
	if hasCurrent {
		if queryMentionsCurrentConversation(query) || len(currentMatches) > 0 {
			return currentMatches
		}
	}
	return filterScheduledTasksByPlainQuery(jobs, query)
}

func currentScheduledTasksForToolQuery(
	jobs []automationdomain.ScheduledTask,
	query string,
	sctx contract.ServerContext,
) ([]automationdomain.ScheduledTask, bool) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return nil, false
	}
	scoped := filterScheduledTasksByCurrentContext(jobs, current)
	if !queryMentionsCurrentConversation(query) {
		return filterScheduledTasksByPlainQuery(scoped, query), true
	}
	remainder := stripCurrentConversationTerms(query)
	if strings.TrimSpace(remainder) == "" {
		return scoped, true
	}
	return filterScheduledTasksByPlainQuery(scoped, remainder), true
}

func bestMatchingCurrentScheduledTasksForToolQuery(
	jobs []automationdomain.ScheduledTask,
	query string,
	sctx contract.ServerContext,
) ([]automationdomain.ScheduledTask, bool) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return nil, false
	}
	scoped := filterScheduledTasksByCurrentContext(jobs, current)
	if !queryMentionsCurrentConversation(query) {
		return automationexec.BestMatchingScheduledTasks(scoped, query), true
	}
	remainder := stripCurrentConversationTerms(query)
	if strings.TrimSpace(remainder) == "" {
		return scoped, true
	}
	return automationexec.BestMatchingScheduledTasks(scoped, remainder), true
}

func filterScheduledTasksByPlainQuery(jobs []automationdomain.ScheduledTask, query string) []automationdomain.ScheduledTask {
	matches := make([]automationdomain.ScheduledTask, 0, len(jobs))
	for _, job := range jobs {
		if automationexec.ScheduledTaskMatchesQuery(job, query) {
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

func filterScheduledTasksByCurrentExternalContext(
	jobs []automationdomain.ScheduledTask,
	current currentTaskContext,
) []automationdomain.ScheduledTask {
	if !current.external {
		return nil
	}
	return filterScheduledTasksByCurrentContext(jobs, current)
}

func filterScheduledTasksByCurrentContext(
	jobs []automationdomain.ScheduledTask,
	current currentTaskContext,
) []automationdomain.ScheduledTask {
	matches := make([]automationdomain.ScheduledTask, 0, len(jobs))
	for _, job := range jobs {
		if scheduledTaskMatchesCurrentContext(job, current) {
			matches = append(matches, job)
		}
	}
	return matches
}

func scheduledTaskMatchesCurrentContext(job automationdomain.ScheduledTask, current currentTaskContext) bool {
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

func taskEventMatchesCurrentContext(event automationdomain.ScheduledTaskEvent, current currentTaskContext) bool {
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
