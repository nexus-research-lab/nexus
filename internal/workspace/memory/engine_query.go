package memory

import (
	"context"
	"sort"
	"strings"
)

var closedStatuses = map[string]struct{}{
	"ignored":  {},
	"deleted":  {},
	"resolved": {},
}

// List 返回结构化记忆条目。
func (e *Engine) List(ctx context.Context, options MemoryListOptions) ([]MemoryItem, error) {
	if e == nil || !e.options.Enabled {
		return nil, nil
	}
	if options.Limit <= 0 {
		options.Limit = defaultListLimit
	}
	entries, err := e.repository.ListEntries(options.Limit)
	if err != nil {
		return nil, err
	}
	statuses := normalizeStatusSet(options.Statuses)
	scopeFilter := strings.TrimSpace(options.Scope)
	items := make([]MemoryItem, 0, len(entries))
	for _, entry := range entries {
		if len(statuses) > 0 {
			if _, ok := statuses[entry.Status()]; !ok {
				continue
			}
		}
		item := entryToMemoryItem(entry, 0)
		if scopeFilter != "" && item.Scope != scopeFilter {
			continue
		}
		items = append(items, item)
		if len(items) >= options.Limit {
			break
		}
	}
	return items, nil
}

// Search 执行 v1 词法召回。
func (e *Engine) Search(ctx context.Context, scope MemoryScope, request RecallRequest) ([]MemoryItem, error) {
	if e == nil || !e.options.Enabled {
		return nil, nil
	}
	limit := request.MaxResults
	if limit <= 0 {
		limit = e.options.MaxResults
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return nil, nil
	}
	entries, err := e.repository.ListEntries(defaultListLimit)
	if err != nil {
		return nil, err
	}
	items := make([]MemoryItem, 0, len(entries))
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, closed := closedStatuses[entry.Status()]; closed {
			continue
		}
		item := entryToMemoryItem(entry, 0)
		if !scopeCanAccessItem(scope, item) {
			continue
		}
		score := scoreItem(query, scope, item)
		if score < e.options.ScoreThreshold {
			continue
		}
		item.Score = score
		items = append(items, item)
	}
	sortMemoryItems(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func sortMemoryItems(items []MemoryItem) {
	sort.SliceStable(items, func(i int, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].AccessCount != items[j].AccessCount {
			return items[i].AccessCount > items[j].AccessCount
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

// Stats 返回记忆统计。
func (e *Engine) Stats(ctx context.Context) (MemoryStats, error) {
	return e.scopedStats(ctx, "")
}

// ScopedStats 返回指定 scope 下的记忆统计。
func (e *Engine) ScopedStats(ctx context.Context, scope string) (MemoryStats, error) {
	return e.scopedStats(ctx, scope)
}

func (e *Engine) scopedStats(ctx context.Context, scope string) (MemoryStats, error) {
	stats := MemoryStats{
		ByStatus: map[string]int{},
		ByKind:   map[string]int{},
		ByScope:  map[string]int{},
	}
	if e == nil || !e.options.Enabled {
		return stats, nil
	}
	entries, err := e.repository.ListEntries(0)
	if err != nil {
		return stats, err
	}
	scope = strings.TrimSpace(scope)
	for _, entry := range entries {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		item := entryToMemoryItem(entry, 0)
		if scope != "" && item.Scope != scope {
			continue
		}
		stats.Total++
		stats.ByStatus[item.Status]++
		stats.ByKind[item.Kind]++
		if item.Scope != "" {
			stats.ByScope[item.Scope]++
		}
		if item.Status == "candidate" || item.Status == "needs_confirmation" {
			stats.Candidate++
		}
		if item.AccessCount > 1 {
			stats.Accessed++
		}
	}
	if scope == "" {
		if count, err := e.repository.CheckpointCount(); err == nil {
			stats.Checkpointed = count
		}
	} else if checkpoints, err := e.repository.ReadCheckpoints(); err == nil {
		if _, ok := checkpoints.Scopes[scope]; ok {
			stats.Checkpointed = 1
		}
	}
	return stats, nil
}

// SessionSummary 读取会话摘要。
func (e *Engine) SessionSummary(ctx context.Context, sessionKey string) (string, error) {
	if e == nil || !e.options.Enabled {
		return "", nil
	}
	return e.repository.ReadSessionSummary(sessionKey)
}

// StableContext 返回 USER.md/MEMORY.md 这类热记忆。
func (e *Engine) StableContext(ctx context.Context, maxChars int) (string, error) {
	if e == nil || !e.options.Enabled {
		return "", nil
	}
	return e.repository.ReadStableContext(maxChars)
}
