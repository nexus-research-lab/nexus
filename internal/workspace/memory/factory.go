package memory

import (
	"fmt"
	"strings"
	"time"
)

// Factory 负责生成标准化记忆条目。
type Factory struct{}

var (
	defaultFieldsByKind = map[string][]Field{
		"LRN": {
			{Key: "优先级", Value: "medium"},
			{Key: "领域", Value: "general"},
			{Key: "详情", Value: ""},
			{Key: "行动", Value: ""},
			{Key: "来源", Value: "conversation"},
			{Key: "次数", Value: "1"},
			{Key: "标签", Value: ""},
			{Key: "关联", Value: ""},
			{Key: "状态", Value: "pending"},
		},
		"ERR": {
			{Key: "优先级", Value: "high"},
			{Key: "领域", Value: "general"},
			{Key: "错误", Value: ""},
			{Key: "上下文", Value: ""},
			{Key: "修复", Value: ""},
			{Key: "可复现", Value: "unknown"},
			{Key: "次数", Value: "1"},
			{Key: "标签", Value: ""},
			{Key: "关联", Value: ""},
			{Key: "状态", Value: "pending"},
		},
		"FEAT": {
			{Key: "优先级", Value: "medium"},
			{Key: "领域", Value: "general"},
			{Key: "需求", Value: ""},
			{Key: "用户背景", Value: ""},
			{Key: "复杂度", Value: "medium"},
			{Key: "实现", Value: ""},
			{Key: "频率", Value: "first_time"},
			{Key: "状态", Value: "pending"},
		},
		"REF": {
			{Key: "做了什么", Value: ""},
			{Key: "结果", Value: "success"},
			{Key: "反思", Value: ""},
			{Key: "经验", Value: ""},
			{Key: "状态", Value: "pending"},
		},
	}
	confirmationCategories = map[string]struct{}{
		"correction":    {},
		"knowledge_gap": {},
		"best_practice": {},
	}
)

// Create 构建新条目。
func (f Factory) Create(kind string, title string, category string, fields []Field, relatedEntries []*Entry, now time.Time) (*Entry, error) {
	normalizedKind := strings.ToUpper(strings.TrimSpace(kind))
	defaultFields, ok := defaultFieldsByKind[normalizedKind]
	if !ok {
		return nil, fmt.Errorf("不支持的日记类型: %s", kind)
	}
	timestamp := now
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	entry := &Entry{
		ID:        buildEntryID(normalizedKind, timestamp),
		CreatedAt: timestamp,
		Kind:      normalizedKind,
		Title:     strings.TrimSpace(title),
		Category:  strings.TrimSpace(category),
		Fields:    cloneFields(defaultFields),
	}
	for _, field := range fields {
		entry.SetField(field.Key, field.Value)
	}
	f.applyRelatedContext(entry, relatedEntries)
	return entry, nil
}

// InferAutoPromotionTarget 推断自动提升目标。
func (Factory) InferAutoPromotionTarget(entry *Entry) string {
	if entry == nil {
		return ""
	}
	if entry.Kind == "LRN" && entry.Category == "preference" {
		return "soul"
	}
	return ""
}

func (Factory) applyRelatedContext(entry *Entry, relatedEntries []*Entry) {
	if len(relatedEntries) == 0 {
		return
	}
	relatedIDs := make([]string, 0, min(len(relatedEntries), 5))
	maxCount := 1
	for index, item := range relatedEntries {
		if index < 5 {
			relatedIDs = append(relatedIDs, item.ID)
		}
		if item.Count() > maxCount {
			maxCount = item.Count()
		}
	}
	entry.SetRelatedIDs(relatedIDs)
	if entry.FieldValue("次数") != "" {
		entry.SetCount(maxCount + 1)
	}

	// 纠正类学习重复出现时，把状态推进到待确认，
	// 避免系统未经确认直接把短期经验固化成长期规则。
	if entry.Kind == "LRN" && entry.Count() >= 3 {
		if _, ok := confirmationCategories[entry.Category]; ok && entry.Status() == "pending" {
			entry.SetStatus("needs_confirmation")
		}
	}
}

func buildEntryID(kind string, timestamp time.Time) string {
	return fmt.Sprintf("%s-%s-%d", kind, timestamp.Format("20060102-150405"), timestamp.UnixNano())
}

func cloneFields(values []Field) []Field {
	items := make([]Field, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
