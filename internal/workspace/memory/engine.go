package memory

import (
	"context"
	"strings"
	"time"
)

const (
	defaultListLimit        = 200
	sessionSummaryMaxChars  = 1200
	dynamicContextMaxChars  = 1800
	stableContextMaxChars   = 3200
	autoMemoryTitleMaxRunes = 48
)

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
	if scopeKey == "" {
		return CaptureResult{Skipped: true, Reason: "invalid_scope"}, nil
	}
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
