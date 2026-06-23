package memory

import (
	"slices"
	"strings"
)

func (e *Engine) incrementAccessCount(items []MemoryItem) {
	for _, item := range items {
		entryID := strings.TrimSpace(item.EntryID)
		if entryID == "" {
			continue
		}
		_, _ = e.repository.UpdateEntry(entryID, func(entry *Entry) {
			entry.SetCount(entry.Count() + 1)
		})
	}
}

func entryToMemoryItem(entry *Entry, score float64) MemoryItem {
	if entry == nil {
		return MemoryItem{}
	}
	return MemoryItem{
		EntryID:     entry.ID,
		Path:        entry.Path,
		Kind:        entry.Kind,
		Category:    entry.Category,
		Title:       entry.Title,
		Content:     entryContent(entry),
		Status:      entry.Status(),
		Priority:    strings.TrimSpace(entry.FieldValue("优先级")),
		Source:      strings.TrimSpace(entry.FieldValue("来源")),
		Scope:       strings.TrimSpace(entry.FieldValue("Scope")),
		SessionKey:  strings.TrimSpace(entry.FieldValue("会话")),
		RoundID:     strings.TrimSpace(entry.FieldValue("RoundID")),
		AccessCount: entry.Count(),
		Score:       score,
		CreatedAt:   entry.CreatedAt,
		Fields:      slices.Clone(entry.Fields),
	}
}

func entryContent(entry *Entry) string {
	if key := primaryContentField(entry); key != "" {
		if value := strings.TrimSpace(entry.FieldValue(key)); value != "" {
			return value
		}
	}
	for _, key := range []string{"详情", "行动", "做了什么", "结果", "经验", "需求", "修复", "错误", "反思"} {
		value := strings.TrimSpace(entry.FieldValue(key))
		if value != "" {
			return value
		}
	}
	return strings.TrimSpace(entry.Title)
}

func primaryContentField(entry *Entry) string {
	if entry == nil {
		return "详情"
	}
	switch entry.Kind {
	case "REF":
		return "做了什么"
	case "ERR":
		return "错误"
	case "FEAT":
		return "需求"
	default:
		return "详情"
	}
}
