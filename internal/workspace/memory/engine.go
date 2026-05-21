package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	defaultListLimit        = 200
	sessionSummaryMaxChars  = 1200
	dynamicContextMaxChars  = 1800
	stableContextMaxChars   = 3200
	autoMemoryTitleMaxRunes = 48
)

var closedStatuses = map[string]struct{}{
	"ignored":  {},
	"deleted":  {},
	"resolved": {},
}

// Engine 把本地 markdown 记忆升级为可召回、可提交、可治理的运行时接口。
type Engine struct {
	repository *Repository
	service    *Service
	factory    Factory
	options    MemoryOptions
}

// NewEngine 创建运行时记忆引擎。
func NewEngine(workspacePath string, options MemoryOptions) *Engine {
	if options == (MemoryOptions{}) {
		options = DefaultOptions()
	}
	options = options.Normalize()
	return &Engine{
		repository: NewRepository(workspacePath),
		service:    NewService(workspacePath),
		factory:    Factory{},
		options:    options,
	}
}

// BeforeRecall 在本轮请求前召回动态记忆。
func (e *Engine) BeforeRecall(ctx context.Context, scope MemoryScope, request RecallRequest) (MemoryInjection, error) {
	if e == nil || !e.options.Enabled || !e.options.AutoRecall {
		return MemoryInjection{}, nil
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return MemoryInjection{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, e.options.RecallTimeout)
	defer cancel()

	items, err := e.Search(ctx, scope, request)
	if err != nil {
		return MemoryInjection{}, err
	}
	if len(items) == 0 {
		return MemoryInjection{}, nil
	}
	e.incrementAccessCount(items)
	dynamic := renderRelevantMemories(items, dynamicContextMaxChars)
	stable, stableErr := e.repository.ReadStableContext(stableContextMaxChars)
	if stableErr != nil {
		stable = ""
	}
	return MemoryInjection{
		StableSystemContext: stable,
		DynamicUserContext:  dynamic,
		Items:               items,
	}, nil
}

// CommitTurn 在一轮成功对话结束后提交自动记忆候选。
func (e *Engine) CommitTurn(ctx context.Context, scope MemoryScope, turn CommittedTurn) (CaptureResult, error) {
	if e == nil || !e.options.Enabled || !e.options.AutoExtract {
		return CaptureResult{Skipped: true, Reason: "disabled"}, nil
	}
	userText := strings.TrimSpace(turn.UserText)
	assistantText := strings.TrimSpace(turn.AssistantText)
	if userText == "" || assistantText == "" {
		return CaptureResult{Skipped: true, Reason: "empty_turn"}, nil
	}
	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now()
	}
	signal := classifyMemorySignal(userText, assistantText)
	if !signal.ShouldCapture {
		return CaptureResult{Skipped: true, Reason: signal.Reason}, nil
	}
	scopeKey := scope.Key()
	decision, err := NewMemoryScheduler(e.repository).Advance(scopeKey, turn.RoundID, turn.Timestamp, signal.HighImpact)
	if err != nil {
		return CaptureResult{}, err
	}
	if !decision.ShouldCapture {
		return CaptureResult{Skipped: true, Reason: decision.Reason}, nil
	}

	entry, err := e.buildEntry(scope, turn, userText, assistantText, signal)
	if err != nil {
		return CaptureResult{}, err
	}
	path, err := e.repository.AppendEntry(entry)
	if err != nil {
		return CaptureResult{}, err
	}
	entry.Path = path
	sessionPath, err := e.appendSessionSummary(scope, turn, entry)
	if err != nil {
		return CaptureResult{}, err
	}
	item := entryToMemoryItem(entry, 0)
	if sessionPath != "" {
		item.Source = strings.TrimSpace(strings.Join([]string{item.Source, sessionPath}, " "))
	}
	return CaptureResult{Processed: true, Items: []MemoryItem{item}}, nil
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

// Add 手动新增记忆条目。
func (e *Engine) Add(ctx context.Context, scope MemoryScope, input MemoryWriteInput) (MemoryItem, error) {
	if e == nil || !e.options.Enabled {
		return MemoryItem{}, nil
	}
	fields := append([]Field{}, input.Fields...)
	fields = append(fields,
		Field{Key: "详情", Value: input.Content},
		Field{Key: "状态", Value: firstNonEmpty(input.Status, "candidate")},
		Field{Key: "优先级", Value: firstNonEmpty(input.Priority, "medium")},
		Field{Key: "来源", Value: firstNonEmpty(input.Source, "manual")},
		Field{Key: "Scope", Value: firstNonEmpty(input.Scope, scope.Key())},
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

// Stats 返回记忆统计。
func (e *Engine) Stats(ctx context.Context) (MemoryStats, error) {
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
	for _, entry := range entries {
		item := entryToMemoryItem(entry, 0)
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
	if count, err := e.repository.CheckpointCount(); err == nil {
		stats.Checkpointed = count
	}
	return stats, nil
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
			Field{Key: "可复现", Value: "unknown"},
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
		Fields:      append([]Field{}, entry.Fields...),
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

func scoreItem(query string, scope MemoryScope, item MemoryItem) float64 {
	queryTokens := tokenizeText(strings.ToLower(query))
	if len(queryTokens) == 0 {
		return 0
	}
	textTokens := tokenizeText(strings.ToLower(strings.Join([]string{
		item.Title,
		item.Content,
		item.Category,
		item.Source,
		item.Scope,
	}, " ")))
	common := 0
	for token := range queryTokens {
		if _, ok := textTokens[token]; ok {
			common++
		}
	}
	score := float64(common) / float64(len(queryTokens))
	score += scopeBoost(scope, item)
	score += statusBoost(item.Status)
	score += priorityBoost(item.Priority)
	score += recencyBoost(item.CreatedAt)
	if item.AccessCount > 1 {
		score += minFloat(float64(item.AccessCount-1)*0.02, 0.12)
	}
	return score
}

func scopeBoost(scope MemoryScope, item MemoryItem) float64 {
	scopeKey := scope.Key()
	switch {
	case item.Scope == "":
		return 0.02
	case item.Scope == scopeKey:
		return 0.35
	case strings.HasPrefix(item.Scope, string(ScopeKindAgent)+":") && strings.Contains(item.Scope, scope.AgentID):
		return 0.16
	case strings.HasPrefix(item.Scope, string(ScopeKindUser)+":") && strings.Contains(item.Scope, scope.UserID):
		return 0.12
	case strings.HasPrefix(item.Scope, string(ScopeKindRoomShared)+":") && scope.Kind == ScopeKindRoomAgentSession:
		return 0.10
	default:
		return 0
	}
}

func scopeCanAccessItem(scope MemoryScope, item MemoryItem) bool {
	itemScope := strings.TrimSpace(item.Scope)
	if itemScope == "" {
		return true
	}
	scopeKey := scope.Key()
	if itemScope == scopeKey {
		return true
	}
	if strings.HasPrefix(itemScope, string(ScopeKindUser)+":") {
		return scope.UserID != "" && strings.Contains(itemScope, scope.UserID)
	}
	if strings.HasPrefix(itemScope, string(ScopeKindAgent)+":") {
		return scope.AgentID != "" && strings.Contains(itemScope, scope.AgentID)
	}
	if strings.HasPrefix(itemScope, string(ScopeKindDMSession)+":") {
		return scope.Kind == ScopeKindAgent && scope.AgentID != "" && strings.Contains(itemScope, scope.AgentID)
	}
	if strings.HasPrefix(itemScope, string(ScopeKindRoomAgentSession)+":") {
		return scope.Kind == ScopeKindAgent && scope.AgentID != "" && strings.Contains(itemScope, scope.AgentID)
	}
	if strings.HasPrefix(itemScope, string(ScopeKindRoomShared)+":") {
		return scope.Kind == ScopeKindRoomShared || scope.Kind == ScopeKindRoomAgentSession
	}
	return false
}

func statusBoost(status string) float64 {
	switch strings.TrimSpace(status) {
	case "promoted", "active":
		return 0.12
	case "candidate", "needs_confirmation":
		return 0.05
	case "auto", "pending":
		return 0.03
	default:
		return 0
	}
}

func priorityBoost(priority string) float64 {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 0.12
	case "medium":
		return 0.06
	case "low":
		return 0.02
	default:
		return 0
	}
}

func recencyBoost(createdAt time.Time) float64 {
	if createdAt.IsZero() {
		return 0
	}
	age := time.Since(createdAt)
	switch {
	case age <= 24*time.Hour:
		return 0.10
	case age <= 7*24*time.Hour:
		return 0.07
	case age <= 30*24*time.Hour:
		return 0.04
	case age <= 180*24*time.Hour:
		return 0.02
	default:
		return 0
	}
}

func renderRelevantMemories(items []MemoryItem, maxChars int) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{"<relevant-memories>"}
	total := len(lines[0])
	for _, item := range items {
		line := fmt.Sprintf(
			"- [%s] %s：%s (status=%s, scope=%s, access_count=%d)",
			item.EntryID,
			item.Title,
			truncateRunes(item.Content, 220),
			item.Status,
			item.Scope,
			item.AccessCount,
		)
		if total+len(line) > maxChars {
			break
		}
		lines = append(lines, line)
		total += len(line)
	}
	lines = append(lines, "</relevant-memories>")
	return strings.Join(lines, "\n")
}

func normalizeStatusSet(statuses []string) map[string]struct{} {
	result := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		value := strings.TrimSpace(status)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
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
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
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

func summarizeTitle(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return "自动记忆"
	}
	return truncateRunes(text, autoMemoryTitleMaxRunes)
}

func truncateRunes(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if clean := strings.TrimSpace(value); clean != "" {
			return clean
		}
	}
	return ""
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

// ExtractMessageText 把 runtime message 转成可用于记忆的正文。
func ExtractMessageText(message protocol.Message) string {
	if len(message) == 0 {
		return ""
	}
	if value := normalizeMessageString(message["text"]); value != "" {
		return value
	}
	if value := normalizeMessageString(message["content"]); value != "" {
		return value
	}
	if content, ok := message["content"].([]any); ok {
		parts := make([]string, 0, len(content))
		for _, block := range content {
			if text := extractContentBlockText(block); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if content, ok := message["content"].([]map[string]any); ok {
		parts := make([]string, 0, len(content))
		for _, block := range content {
			if text := extractContentBlockText(block); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func extractContentBlockText(block any) string {
	value, ok := block.(map[string]any)
	if !ok {
		return normalizeMessageString(block)
	}
	for _, key := range []string{"text", "content", "result"} {
		if text := normalizeMessageString(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func normalizeMessageString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func sortedMapKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
