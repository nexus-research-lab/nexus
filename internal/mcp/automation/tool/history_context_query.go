package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func searchTaskHistoryForToolQuery(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	input protocol.CronTaskHistorySearchInput,
) ([]protocol.CronTaskHistoryItem, error) {
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return svc.SearchTaskHistory(ctx, input)
	}
	mentionsCurrent := queryMentionsCurrentConversation(input.Query)
	limit := normalizedTaskHistoryToolLimit(input.Limit)
	items := make([]protocol.CronTaskHistoryItem, 0, limit)
	seen := map[string]bool{}
	if input.IncludeActive {
		jobs, err := svc.ListTasks(ctx, strings.TrimSpace(input.AgentID))
		if err != nil {
			return nil, err
		}
		for _, job := range currentCronJobsForHistoryQuery(jobs, input.Query, sctx) {
			if appendTaskHistoryItem(&items, seen, cronJobTaskHistoryItem(job), limit) {
				return items, nil
			}
		}
	}
	if input.IncludeDeleted {
		deletedInput := input
		if mentionsCurrent {
			deletedInput.Query = stripCurrentConversationTerms(input.Query)
		}
		deletedInput.IncludeActive = false
		deletedInput.IncludeDeleted = true
		deletedInput.Limit = 50
		deleted, err := svc.SearchTaskHistory(ctx, deletedInput)
		if err != nil {
			return nil, err
		}
		filtered, err := filterTaskHistoryItemsByCurrentContext(ctx, svc, current, deleted)
		if err != nil {
			return nil, err
		}
		for _, item := range filtered {
			if appendTaskHistoryItem(&items, seen, item, limit) {
				return items, nil
			}
		}
	}
	if len(items) > 0 || mentionsCurrent {
		return items, nil
	}
	return svc.SearchTaskHistory(ctx, input)
}

func currentCronJobsForHistoryQuery(
	jobs []protocol.CronJob,
	query string,
	sctx contract.ServerContext,
) []protocol.CronJob {
	matches, hasCurrent := currentCronJobsForToolQuery(jobs, query, sctx)
	if !hasCurrent {
		return nil
	}
	return matches
}

func requireCurrentConversationTaskHistoryScopeForQuery(
	scopedCtx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	agentID string,
	query string,
) (taskHistoryScope, bool, error) {
	mentionsCurrent := queryMentionsCurrentConversation(query)
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok {
		return taskHistoryScope{}, false, nil
	}
	jobs, err := svc.ListTasks(scopedCtx, agentID)
	if err != nil {
		return taskHistoryScope{}, true, err
	}
	matches, _ := bestMatchingCurrentCronJobsForToolQuery(jobs, query, sctx)
	switch len(matches) {
	case 1:
		job := matches[0]
		if err = ensureTaskBelongsToCaller(sctx, job.JobID, strings.TrimSpace(job.AgentID)); err != nil {
			return taskHistoryScope{}, true, err
		}
		return taskHistoryScope{Context: scopedCtx, JobID: strings.TrimSpace(job.JobID)}, true, nil
	case 0:
		scope, handled, err := requireDeletedCurrentConversationTaskHistoryScopeForQuery(scopedCtx, svc, current, sctx, agentID, query)
		if handled || mentionsCurrent {
			return scope, true, err
		}
		return taskHistoryScope{}, false, nil
	default:
		return taskHistoryScope{}, true, fmt.Errorf("query %q matched multiple current scheduled tasks; ask the user to choose one job_id: %s", strings.TrimSpace(query), describeCronJobCandidates(matches, 5))
	}
}

func requireDeletedCurrentConversationTaskHistoryScopeForQuery(
	scopedCtx context.Context,
	svc contract.Service,
	current currentTaskContext,
	sctx contract.ServerContext,
	agentID string,
	query string,
) (taskHistoryScope, bool, error) {
	matches, err := deletedCurrentConversationTaskHistoryMatches(scopedCtx, svc, current, agentID, query)
	if err != nil {
		return taskHistoryScope{}, true, err
	}
	if len(matches) == 0 && !queryMentionsCurrentConversation(query) {
		return taskHistoryScope{}, false, nil
	}
	switch len(matches) {
	case 0:
		return taskHistoryScope{}, true, fmt.Errorf("no scheduled task history in current conversation matched query %q", strings.TrimSpace(query))
	case 1:
		if err = ensureTaskBelongsToCaller(sctx, matches[0].JobID, strings.TrimSpace(matches[0].AgentID)); err != nil {
			return taskHistoryScope{}, true, err
		}
		return taskHistoryScope{Context: scopedCtx, JobID: strings.TrimSpace(matches[0].JobID)}, true, nil
	default:
		return taskHistoryScope{}, true, fmt.Errorf("query %q matched multiple current conversation scheduled task history candidates; ask the user to choose one job_id: %s", strings.TrimSpace(query), describeTaskHistoryCandidates(matches, 5))
	}
}

func deletedCurrentConversationTaskHistoryMatches(
	scopedCtx context.Context,
	svc contract.Service,
	current currentTaskContext,
	agentID string,
	query string,
) ([]protocol.CronTaskHistoryItem, error) {
	searchQuery := query
	if queryMentionsCurrentConversation(query) {
		searchQuery = stripCurrentConversationTerms(query)
	}
	items, err := svc.SearchTaskHistory(scopedCtx, protocol.CronTaskHistorySearchInput{
		Query:          searchQuery,
		AgentID:        agentID,
		IncludeActive:  false,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		return nil, err
	}
	return filterTaskHistoryItemsByCurrentContext(scopedCtx, svc, current, items)
}

func filterTaskHistoryItemsByCurrentContext(
	ctx context.Context,
	svc contract.Service,
	current currentTaskContext,
	items []protocol.CronTaskHistoryItem,
) ([]protocol.CronTaskHistoryItem, error) {
	matches := make([]protocol.CronTaskHistoryItem, 0, len(items))
	for _, item := range items {
		events, err := svc.ListTaskEvents(ctx, item.JobID, 50)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if taskEventMatchesCurrentContext(event, current) {
				matches = append(matches, item)
				break
			}
		}
	}
	return matches, nil
}

func cronJobTaskHistoryItem(job protocol.CronJob) protocol.CronTaskHistoryItem {
	enabled := job.Enabled
	return protocol.CronTaskHistoryItem{
		JobID:              strings.TrimSpace(job.JobID),
		Name:               strings.TrimSpace(job.Name),
		AgentID:            strings.TrimSpace(job.AgentID),
		Deleted:            false,
		Enabled:            &enabled,
		Running:            job.Running,
		NextRunAt:          job.NextRunAt,
		LastRunAt:          job.LastRunAt,
		LastRunStatus:      strings.TrimSpace(job.LastRunStatus),
		LastDeliveryStatus: strings.TrimSpace(job.LastDeliveryStatus),
	}
}

func appendTaskHistoryItem(items *[]protocol.CronTaskHistoryItem, seen map[string]bool, item protocol.CronTaskHistoryItem, limit int) bool {
	jobID := strings.TrimSpace(item.JobID)
	if jobID == "" || seen[jobID] {
		return false
	}
	seen[jobID] = true
	*items = append(*items, item)
	return len(*items) >= limit
}

func normalizedTaskHistoryToolLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return min(limit, 50)
}
