package automation

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ExecutionObservation 是自动化执行轮次的最终观测结果。
type ExecutionObservation struct {
	Status        string
	SessionID     *string
	MessageCount  int
	ErrorMessage  *string
	AssistantText string
	ResultText    string
}

// ExecutionSink 实现 permission.Sender，用于后台自动化观察 runtime 事件。
type ExecutionSink struct {
	key    string
	events chan protocol.EventMessage

	mu     sync.RWMutex
	closed bool
}

// NewExecutionSink 创建自动化执行事件观察器。
func NewExecutionSink(key string) *ExecutionSink {
	return &ExecutionSink{
		key:    key,
		events: make(chan protocol.EventMessage, 256),
	}
}

func (s *ExecutionSink) Key() string {
	return s.key
}

func (s *ExecutionSink) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *ExecutionSink) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *ExecutionSink) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return nil
	}

	select {
	case s.events <- event:
	default:
		// 自动化观察器只需要终态与关键消息，缓冲打满时丢弃最旧实时事件，
		// 避免后台任务因为无人消费的中间 token 卡死。
		select {
		case <-s.events:
		default:
		}
		select {
		case s.events <- event:
		default:
		}
	}
	return nil
}

func (s *ExecutionSink) WaitForRound(ctx context.Context, roundID string) ExecutionObservation {
	observer := roundObserver{
		roundID: strings.TrimSpace(roundID),
		result:  ExecutionObservation{Status: types.RunStatusRunning},
	}
	for {
		select {
		case <-ctx.Done():
			return observer.cancel(ctx.Err())
		case event := <-s.events:
			if observer.observe(event) {
				return observer.result
			}
		}
	}
}

type roundObserver struct {
	roundID string
	result  ExecutionObservation
}

func (o *roundObserver) observe(event protocol.EventMessage) bool {
	switch event.EventType {
	case protocol.EventTypeMessage:
		o.observeMessage(event.Data)
		return false
	case protocol.EventTypeError:
		o.observeError(event.Data)
		return true
	case protocol.EventTypeRoundStatus:
		return o.observeRoundStatus(event.Data)
	default:
		return false
	}
}

func (o *roundObserver) observeMessage(payload map[string]any) {
	if !o.matchesRound(payload) {
		return
	}
	o.result.MessageCount++
	if sessionID := strings.TrimSpace(anyString(payload["session_id"])); sessionID != "" {
		o.result.SessionID = &sessionID
	}
	switch strings.TrimSpace(anyString(payload["role"])) {
	case "assistant":
		o.observeAssistantMessage(payload)
	case "result":
		applyResultPayload(&o.result, payload)
	}
}

func (o *roundObserver) observeAssistantMessage(payload map[string]any) {
	if text := strings.TrimSpace(extractTextContent(payload["content"])); text != "" {
		o.result.AssistantText = text
	}
	if summary, ok := payload["result_summary"].(map[string]any); ok {
		applyResultPayload(&o.result, summary)
	}
}

func (o *roundObserver) observeError(payload map[string]any) {
	if message := strings.TrimSpace(anyString(payload["message"])); message != "" {
		o.result.ErrorMessage = &message
	}
	o.result.Status = types.RunStatusFailed
}

func (o *roundObserver) observeRoundStatus(payload map[string]any) bool {
	if !o.matchesRound(payload) || !anyBool(payload["is_terminal"]) {
		return false
	}
	status := strings.TrimSpace(anyString(payload["status"]))
	if status == "finished" && o.result.Status != types.RunStatusRunning {
		return true
	}
	runStatus, known := roundTerminalRunStatuses[status]
	if known {
		o.result.Status = runStatus
		return true
	}
	o.result.Status = types.RunStatusFailed
	o.captureTerminalError(payload)
	return true
}

var roundTerminalRunStatuses = map[string]string{
	"finished":    types.RunStatusSucceeded,
	"interrupted": types.RunStatusCancelled,
}

func (o *roundObserver) captureTerminalError(payload map[string]any) {
	if o.result.ErrorMessage != nil {
		return
	}
	if message := strings.TrimSpace(anyString(payload["result_subtype"])); message != "" {
		o.result.ErrorMessage = &message
	}
}

func (o *roundObserver) matchesRound(payload map[string]any) bool {
	return strings.TrimSpace(anyString(payload["round_id"])) == o.roundID
}

func (o *roundObserver) cancel(err error) ExecutionObservation {
	message := err.Error()
	o.result.Status = types.RunStatusCancelled
	o.result.ErrorMessage = &message
	return o.result
}

func applyResultPayload(observation *ExecutionObservation, payload map[string]any) {
	if resultText := strings.TrimSpace(anyString(payload["result"])); resultText != "" {
		observation.ResultText = resultText
	}
	if message := permissionDenialErrorMessage(payload, observation.ResultText); message != "" {
		observation.Status = types.RunStatusFailed
		observation.ErrorMessage = &message
		return
	}
	if message := resultErrorsMessage(payload, observation.ResultText); message != "" {
		observation.Status = types.RunStatusFailed
		observation.ErrorMessage = &message
		return
	}
	switch strings.TrimSpace(anyString(payload["subtype"])) {
	case "success", "":
		observation.Status = types.RunStatusSucceeded
	case "interrupted":
		observation.Status = types.RunStatusCancelled
	default:
		observation.Status = types.RunStatusFailed
		message := strings.TrimSpace(anyString(payload["result"]))
		if message != "" {
			observation.ErrorMessage = &message
		}
	}
}

func permissionDenialErrorMessage(payload map[string]any, resultText string) string {
	tools := permissionDenialToolNames(payload["permission_denials"])
	if len(tools) == 0 {
		return ""
	}
	if resultText := strings.TrimSpace(resultText); resultText != "" {
		return resultText
	}
	return "定时任务后台运行被权限策略拒绝，未授权工具: " + strings.Join(tools, ", ")
}

func permissionDenialToolNames(value any) []string {
	seen := map[string]struct{}{}
	result := []string{}
	appendName := func(raw any) {
		name := strings.TrimSpace(anyString(raw))
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	appendPayload := func(payload map[string]any) {
		appendName(payload["tool_name"])
	}
	switch typed := value.(type) {
	case []map[string]any:
		for _, item := range typed {
			appendPayload(item)
		}
	case []any:
		for _, raw := range typed {
			payload, ok := raw.(map[string]any)
			if ok {
				appendPayload(payload)
			}
		}
	}
	return result
}

func resultErrorsMessage(payload map[string]any, resultText string) string {
	errors := resultErrorStrings(payload["errors"])
	if len(errors) == 0 {
		return ""
	}
	if resultText := strings.TrimSpace(resultText); resultText != "" {
		return resultText
	}
	return strings.Join(errors, "; ")
}

func resultErrorStrings(value any) []string {
	result := []string{}
	appendError := func(raw any) {
		text := strings.TrimSpace(anyString(raw))
		if text != "" {
			result = append(result, text)
		}
	}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			appendError(item)
		}
	case []any:
		for _, item := range typed {
			appendError(item)
		}
	}
	return result
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func extractTextContent(value any) string {
	switch typed := value.(type) {
	case []map[string]any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if strings.TrimSpace(anyString(item["type"])) != "text" {
				continue
			}
			text := strings.TrimSpace(anyString(item["text"]))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			item, ok := raw.(map[string]any)
			if !ok || strings.TrimSpace(anyString(item["type"])) != "text" {
				continue
			}
			text := strings.TrimSpace(anyString(item["text"]))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func anyBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}

// WaitTimeout 返回自动化执行观察的等待时长。
func WaitTimeout(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 30 * time.Minute
	}
	return duration
}
