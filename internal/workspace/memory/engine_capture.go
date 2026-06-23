package memory

import (
	"fmt"
	"slices"
	"strings"
)

func (e *Engine) buildEntry(
	scope MemoryScope,
	turn CommittedTurn,
	userText string,
	assistantText string,
	signal memorySignal,
) (*Entry, error) {
	status := "auto"
	kind := "LRN"
	category := firstNonEmpty(signal.Category, "observation")
	content := durableMemoryText(userText)
	title := summarizeTitle(content)
	if signal.HighImpact {
		status = "candidate"
		category = "preference"
	} else if signal.Category == "incident" {
		kind = "ERR"
	} else if signal.Category == "todo" {
		kind = "FEAT"
	}
	fields := []Field{
		{Key: "状态", Value: status},
		{Key: "来源", Value: "auto_extract"},
		{Key: "Scope", Value: scope.Key()},
		{Key: "会话", Value: firstNonEmpty(turn.SessionKey, scope.SessionKey)},
		{Key: "RoundID", Value: turn.RoundID},
		{Key: "AgentID", Value: firstNonEmpty(turn.AgentID, scope.AgentID)},
		{Key: "RoomID", Value: firstNonEmpty(turn.RoomID, scope.RoomID)},
		{Key: "ConversationID", Value: firstNonEmpty(turn.ConversationID, scope.ConversationID)},
		{Key: "提取原因", Value: signal.Reason},
	}
	switch kind {
	case "ERR":
		fields = append(fields,
			Field{Key: "优先级", Value: "high"},
			Field{Key: "错误", Value: truncateRunes(content, 700)},
			Field{Key: "修复", Value: truncateRunes(assistantText, 700)},
		)
	case "FEAT":
		fields = append(fields,
			Field{Key: "优先级", Value: "medium"},
			Field{Key: "需求", Value: truncateRunes(content, 700)},
			Field{Key: "实现", Value: truncateRunes(assistantText, 500)},
			Field{Key: "频率", Value: "follow_up"},
		)
	default:
		priority := "medium"
		if signal.HighImpact {
			priority = "high"
		}
		fields = append(fields,
			Field{Key: "优先级", Value: priority},
			Field{Key: "领域", Value: "general"},
			Field{Key: "详情", Value: truncateRunes(content, 900)},
			Field{Key: "证据", Value: truncateRunes(assistantText, 500)},
		)
	}
	return e.factory.Create(kind, title, category, fields, nil, turn.Timestamp)
}

func (e *Engine) appendSessionSummary(scope MemoryScope, turn CommittedTurn, entry *Entry) (string, error) {
	sessionKey := firstNonEmpty(turn.SessionKey, scope.SessionKey)
	if sessionKey == "" {
		return "", nil
	}
	content := fmt.Sprintf(
		"## %s\n\n- Entry: %s\n- Scope: %s\n- User: %s\n- Assistant: %s",
		entry.CreatedAt.Format("2006-01-02 15:04"),
		entry.ID,
		scope.Key(),
		truncateRunes(strings.TrimSpace(turn.UserText), 260),
		truncateRunes(strings.TrimSpace(turn.AssistantText), 360),
	)
	return e.repository.AppendSessionSummary(sessionKey, truncateRunes(content, sessionSummaryMaxChars))
}

type memorySignal struct {
	ShouldCapture bool
	HighImpact    bool
	Category      string
	Reason        string
}

func classifyMemorySignal(userText string, assistantText string) memorySignal {
	if isHighImpactMemory(userText) {
		return memorySignal{
			ShouldCapture: true,
			HighImpact:    true,
			Category:      "preference",
			Reason:        "high_impact",
		}
	}
	combined := strings.ToLower(strings.Join([]string{userText, assistantText}, "\n"))
	switch {
	case containsAny(combined, durableDecisionKeywords()):
		return memorySignal{ShouldCapture: true, Category: "decision", Reason: "durable_decision"}
	case containsAny(combined, durableProcessKeywords()):
		return memorySignal{ShouldCapture: true, Category: "workflow", Reason: "durable_workflow"}
	case containsAny(combined, durableTodoKeywords()):
		return memorySignal{ShouldCapture: true, Category: "todo", Reason: "durable_todo"}
	case containsAny(combined, durableIncidentKeywords()):
		return memorySignal{ShouldCapture: true, Category: "incident", Reason: "durable_incident"}
	default:
		return memorySignal{ShouldCapture: false, Reason: "low_signal"}
	}
}

func isHighImpactMemory(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"记住", "以后", "默认", "偏好", "不要", "别", "必须", "规则", "习惯",
		"remember", "always", "never", "prefer", "preference", "rule", "default",
	}
	return slices.ContainsFunc(keywords, func(keyword string) bool {
		return strings.Contains(lower, keyword)
	})
}

func containsAny(text string, keywords []string) bool {
	return slices.ContainsFunc(keywords, func(keyword string) bool {
		return strings.Contains(text, keyword)
	})
}

func durableDecisionKeywords() []string {
	return []string{
		"结论", "决定", "约定", "共识", "原则", "边界", "职责", "验收",
		"decision", "decided", "agreed", "agreement", "principle", "boundary", "acceptance",
	}
}

func durableProcessKeywords() []string {
	return []string{
		"规范", "目录结构", "命名", "发布流程", "测试策略",
		"convention", "workflow", "naming",
	}
}

func durableTodoKeywords() []string {
	return []string{
		"待办", "下一步", "后续推进", "阻塞", "风险", "里程碑",
		"todo", "follow-up", "next step", "blocker", "risk", "milestone",
	}
}

func durableIncidentKeywords() []string {
	return []string{
		"根因", "复现", "回归", "数据迁移", "schema", "panic", "deadlock", "race condition",
		"root cause", "reproduce", "regression", "migration",
	}
}

func durableMemoryText(text string) string {
	text = strings.TrimSpace(text)
	prefixes := []string{
		"结论：", "结论:", "决定：", "决定:", "约定：", "约定:",
		"共识：", "共识:", "原则：", "原则:", "根因：", "根因:",
		"待办：", "待办:", "下一步：", "下一步:",
	}
	for {
		next := text
		for _, prefix := range prefixes {
			next = strings.TrimSpace(strings.TrimPrefix(next, prefix))
		}
		if next == text {
			return text
		}
		text = next
	}
}
