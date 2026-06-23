package memory

import (
	"context"
	"slices"
	"strings"
	"time"
)

// Add 手动新增记忆条目。
func (e *Engine) Add(ctx context.Context, scope MemoryScope, input MemoryWriteInput) (MemoryItem, error) {
	if e == nil || !e.options.Enabled {
		return MemoryItem{}, nil
	}
	scopeKey := firstNonEmpty(input.Scope, scope.Key())
	if scopeKey == "" {
		return MemoryItem{}, newClientError("scope 不能为空")
	}
	fields := slices.Clone(input.Fields)
	fields = append(fields,
		Field{Key: "详情", Value: input.Content},
		Field{Key: "状态", Value: firstNonEmpty(input.Status, "candidate")},
		Field{Key: "优先级", Value: firstNonEmpty(input.Priority, "medium")},
		Field{Key: "来源", Value: firstNonEmpty(input.Source, "manual")},
		Field{Key: "Scope", Value: scopeKey},
	)
	kind := firstNonEmpty(input.Kind, "LRN")
	category := firstNonEmpty(input.Category, "preference")
	title := firstNonEmpty(input.Title, summarizeTitle(input.Content))
	entry, err := e.factory.Create(kind, title, category, fields, nil, time.Now())
	if err != nil {
		return MemoryItem{}, err
	}
	path, err := e.repository.AppendEntry(entry)
	if err != nil {
		return MemoryItem{}, err
	}
	entry.Path = path
	return entryToMemoryItem(entry, 0), nil
}

// Update 更新记忆条目。
func (e *Engine) Update(ctx context.Context, entryID string, input MemoryWriteInput) (MemoryItem, error) {
	if e == nil || !e.options.Enabled {
		return MemoryItem{}, nil
	}
	entry, err := e.repository.UpdateEntry(entryID, func(entry *Entry) {
		if strings.TrimSpace(input.Title) != "" {
			entry.Title = strings.TrimSpace(input.Title)
		}
		if strings.TrimSpace(input.Category) != "" {
			entry.Category = strings.TrimSpace(input.Category)
		}
		if strings.TrimSpace(input.Content) != "" {
			entry.SetField(primaryContentField(entry), input.Content)
		}
		if strings.TrimSpace(input.Status) != "" {
			entry.SetStatus(input.Status)
		}
		if strings.TrimSpace(input.Priority) != "" {
			entry.SetField("优先级", input.Priority)
		}
		if strings.TrimSpace(input.Source) != "" {
			entry.SetField("来源", input.Source)
		}
		if strings.TrimSpace(input.Scope) != "" {
			entry.SetField("Scope", input.Scope)
		}
		for _, field := range input.Fields {
			entry.SetField(field.Key, field.Value)
		}
	})
	if err != nil {
		return MemoryItem{}, err
	}
	return entryToMemoryItem(entry, 0), nil
}

// Delete 删除记忆条目。
func (e *Engine) Delete(ctx context.Context, entryID string) error {
	if e == nil || !e.options.Enabled {
		return nil
	}
	return e.repository.DeleteEntry(entryID)
}

// Ignore 把候选条目标记为忽略。
func (e *Engine) Ignore(ctx context.Context, entryID string, note string) (MemoryItem, error) {
	item, err := e.service.SetEntryStatus(entryID, "ignored", note)
	if err != nil {
		return MemoryItem{}, err
	}
	entry, err := e.repository.FindEntry(item.EntryID)
	if err != nil {
		return MemoryItem{}, err
	}
	return entryToMemoryItem(entry, 0), nil
}

// Promote 把候选条目提升到长期热记忆。
func (e *Engine) Promote(ctx context.Context, entryID string, target string) (*PromoteResult, error) {
	entry, err := e.repository.FindEntry(entryID)
	if err != nil {
		return nil, err
	}
	return e.service.Promote(firstNonEmpty(target, "memory"), buildPromotionContent(entry), entry.Title, entry.ID)
}

// Cleanup 清理已无结构化条目引用的 session 摘要和 checkpoint。
func (e *Engine) Cleanup(ctx context.Context) (MemoryCleanupResult, error) {
	if e == nil || !e.options.Enabled {
		return MemoryCleanupResult{}, nil
	}
	entries, err := e.repository.ListEntries(0)
	if err != nil {
		return MemoryCleanupResult{}, err
	}
	entryIDs := make(map[string]struct{}, len(entries))
	scopes := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if ctx.Err() != nil {
			return MemoryCleanupResult{}, ctx.Err()
		}
		entryID := strings.TrimSpace(entry.ID)
		if entryID != "" {
			entryIDs[entryID] = struct{}{}
		}
		scope := strings.TrimSpace(entry.FieldValue("Scope"))
		if scope != "" {
			scopes[scope] = struct{}{}
		}
	}
	return e.repository.CleanupOrphans(entryIDs, scopes)
}
