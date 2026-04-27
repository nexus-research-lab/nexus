package memory

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Field 表示记忆条目的有序字段。
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ReviewItem 表示近期回顾条目的摘要。
type ReviewItem struct {
	EntryID  string `json:"entry_id"`
	Path     string `json:"path"`
	Headline string `json:"headline"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Count    int    `json:"count"`
}

// Entry 表示单条日记条目。
type Entry struct {
	ID        string    `json:"entry_id"`
	CreatedAt time.Time `json:"created_at"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Category  string    `json:"category,omitempty"`
	Fields    []Field   `json:"fields"`
	Path      string    `json:"path,omitempty"`
}

// Headline 返回条目标题行。
func (e *Entry) Headline() string {
	timestamp := e.CreatedAt.Format("2006-01-02 15:04")
	if e.Kind == "LRN" && strings.TrimSpace(e.Category) != "" {
		return fmt.Sprintf("### %s - [%s] %s: %s", timestamp, e.Kind, e.Category, e.Title)
	}
	return fmt.Sprintf("### %s - [%s] %s", timestamp, e.Kind, e.Title)
}

// Status 返回条目状态。
func (e *Entry) Status() string {
	value := strings.TrimSpace(e.FieldValue("状态"))
	if value == "" {
		return "pending"
	}
	return value
}

// Count 返回累计次数。
func (e *Entry) Count() int {
	value := strings.TrimSpace(e.FieldValue("次数"))
	if value == "" {
		return 1
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 1
	}
	return parsed
}

// RelatedIDs 返回关联条目列表。
func (e *Entry) RelatedIDs() []string {
	raw := strings.TrimSpace(e.FieldValue("关联"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean != "" {
			items = append(items, clean)
		}
	}
	return items
}

// FieldValue 读取指定字段值。
func (e *Entry) FieldValue(key string) string {
	for _, field := range e.Fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}

// SetField 写入字段值。
func (e *Entry) SetField(key string, value string) {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" {
		return
	}
	for index, field := range e.Fields {
		if field.Key == cleanKey {
			e.Fields[index].Value = strings.TrimSpace(value)
			return
		}
	}
	e.Fields = append(e.Fields, Field{
		Key:   cleanKey,
		Value: strings.TrimSpace(value),
	})
}

// SetStatus 更新状态。
func (e *Entry) SetStatus(status string) {
	e.SetField("状态", status)
}

// SetCount 更新次数。
func (e *Entry) SetCount(count int) {
	if count <= 0 {
		count = 1
	}
	e.SetField("次数", strconv.Itoa(count))
}

// SetRelatedIDs 更新关联条目。
func (e *Entry) SetRelatedIDs(ids []string) {
	e.SetField("关联", strings.Join(ids, ", "))
}

// Markdown 渲染为标准 markdown。
func (e *Entry) Markdown() string {
	lines := []string{e.Headline(), fmt.Sprintf("*   **ID**: %s", e.ID)}
	for _, field := range e.Fields {
		if field.Key == "ID" || strings.TrimSpace(field.Value) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("*   **%s**: %s", field.Key, field.Value))
	}
	return strings.Join(lines, "\n")
}

// ReviewItem 返回回顾摘要。
func (e *Entry) ReviewItem() ReviewItem {
	return ReviewItem{
		EntryID:  e.ID,
		Path:     e.Path,
		Headline: e.Headline(),
		Kind:     e.Kind,
		Status:   e.Status(),
		Count:    e.Count(),
	}
}
