package automation

import (
	"context"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// SearchTaskHistory 按名称、job_id、动作或审计 detail 搜索当前与历史任务候选。
func (s *Service) SearchTaskHistory(ctx context.Context, input automationdomain.CronTaskHistorySearchInput) ([]automationdomain.CronTaskHistoryItem, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	normalized := normalizeTaskHistorySearchInput(input)
	items := make([]automationdomain.CronTaskHistoryItem, 0, normalized.Limit)
	seen := map[string]bool{}
	if normalized.IncludeActive {
		active, err := s.searchActiveTaskHistory(ctx, normalized)
		if err != nil {
			return nil, err
		}
		for _, item := range active {
			if appendTaskHistoryItem(&items, seen, item, normalized.Limit) {
				return items, nil
			}
		}
	}
	if normalized.IncludeDeleted {
		deleted, err := s.searchDeletedTaskHistory(ctx, normalized, seen)
		if err != nil {
			return nil, err
		}
		for _, item := range deleted {
			if appendTaskHistoryItem(&items, seen, item, normalized.Limit) {
				return items, nil
			}
		}
	}
	return items, nil
}

func normalizeTaskHistorySearchInput(input automationdomain.CronTaskHistorySearchInput) automationdomain.CronTaskHistorySearchInput {
	result := input
	result.Query = strings.TrimSpace(result.Query)
	result.AgentID = strings.TrimSpace(result.AgentID)
	if result.Limit <= 0 || result.Limit > 50 {
		result.Limit = 20
	}
	if !result.IncludeActive && !result.IncludeDeleted {
		result.IncludeActive = true
		result.IncludeDeleted = true
	}
	return result
}

func (s *Service) searchActiveTaskHistory(ctx context.Context, input automationdomain.CronTaskHistorySearchInput) ([]automationdomain.CronTaskHistoryItem, error) {
	jobs, err := s.ListTasks(ctx, input.AgentID)
	if err != nil {
		return nil, err
	}
	items := make([]automationdomain.CronTaskHistoryItem, 0, len(jobs))
	for _, job := range jobs {
		if input.Query != "" && !automationexec.CronJobMatchesQuery(job, input.Query) {
			continue
		}
		enabled := job.Enabled
		items = append(items, automationdomain.CronTaskHistoryItem{
			JobID:              job.JobID,
			Name:               job.Name,
			AgentID:            job.AgentID,
			Deleted:            false,
			Enabled:            &enabled,
			Running:            job.Running,
			NextRunAt:          cloneTimePointer(job.NextRunAt),
			LastRunAt:          cloneTimePointer(job.LastRunAt),
			LastRunStatus:      strings.TrimSpace(job.LastRunStatus),
			LastDeliveryStatus: strings.TrimSpace(job.LastDeliveryStatus),
		})
	}
	return items, nil
}

func (s *Service) searchDeletedTaskHistory(
	ctx context.Context,
	input automationdomain.CronTaskHistorySearchInput,
	seen map[string]bool,
) ([]automationdomain.CronTaskHistoryItem, error) {
	ownerUserID, _ := scopedOwnerUserID(ctx)
	events, err := s.searchTaskHistoryEvents(ctx, ownerUserID, input)
	if err != nil {
		return nil, err
	}
	itemsByJob := map[string]*automationdomain.CronTaskHistoryItem{}
	order := make([]string, 0)
	for _, event := range events {
		jobID := strings.TrimSpace(event.JobID)
		if jobID == "" || seen[jobID] {
			continue
		}
		item := itemsByJob[jobID]
		if item == nil {
			item = &automationdomain.CronTaskHistoryItem{
				JobID:         jobID,
				Name:          firstNonEmpty(stringFromTaskEventDetail(event.Detail, "name"), jobID),
				AgentID:       strings.TrimSpace(event.AgentID),
				Deleted:       true,
				LatestAction:  strings.TrimSpace(event.Action),
				LatestEventAt: cloneTimePointer(&event.CreatedAt),
			}
			itemsByJob[jobID] = item
			order = append(order, jobID)
		}
		if item.Name == jobID {
			item.Name = firstNonEmpty(stringFromTaskEventDetail(event.Detail, "name"), item.Name)
		}
		if strings.TrimSpace(item.AgentID) == "" {
			item.AgentID = strings.TrimSpace(event.AgentID)
		}
		if event.Action == automationdomain.TaskEventActionDelete && item.DeletedAt == nil {
			item.DeletedAt = cloneTimePointer(&event.CreatedAt)
		}
	}
	items := make([]automationdomain.CronTaskHistoryItem, 0, len(order))
	for _, jobID := range order {
		item := itemsByJob[jobID]
		runs, err := s.repository.ListRunsByJob(ctx, ownerUserID, jobID)
		if err != nil {
			return nil, err
		}
		applyTaskHistoryRunSummary(item, runs)
		items = append(items, *item)
	}
	return items, nil
}

func (s *Service) searchTaskHistoryEvents(
	ctx context.Context,
	ownerUserID string,
	input automationdomain.CronTaskHistorySearchInput,
) ([]automationdomain.CronTaskEvent, error) {
	queries := automationexec.QueryVariants(input.Query)
	events := make([]automationdomain.CronTaskEvent, 0)
	seen := map[string]bool{}
	limit := input.Limit * 5
	for _, query := range queries {
		items, err := s.repository.SearchTaskEvents(ctx, ownerUserID, input.AgentID, query, limit)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			eventID := strings.TrimSpace(item.EventID)
			if eventID == "" || seen[eventID] {
				continue
			}
			seen[eventID] = true
			events = append(events, item)
		}
	}
	return events, nil
}

func applyTaskHistoryRunSummary(item *automationdomain.CronTaskHistoryItem, runs []automationdomain.CronRun) {
	if item == nil {
		return
	}
	item.RunCount = len(runs)
	if len(runs) == 0 {
		return
	}
	latest := runs[0]
	item.LastRunAt = cloneTimePointer(cronRunReportTimePointer(latest))
	item.LastRunStatus = strings.TrimSpace(latest.Status)
	item.LastDeliveryStatus = deriveCronRunDeliveryStatus(latest)
}

func cronRunReportTimePointer(run automationdomain.CronRun) *time.Time {
	when := cronRunReportTime(run)
	if when.IsZero() {
		return nil
	}
	return &when
}

func appendTaskHistoryItem(items *[]automationdomain.CronTaskHistoryItem, seen map[string]bool, item automationdomain.CronTaskHistoryItem, limit int) bool {
	jobID := strings.TrimSpace(item.JobID)
	if jobID == "" || seen[jobID] {
		return false
	}
	seen[jobID] = true
	*items = append(*items, item)
	return len(*items) >= limit
}
