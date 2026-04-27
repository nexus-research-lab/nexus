package automation

import (
	"context"
	"strings"
	"sync"
	"time"

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
	normalizedRoundID := strings.TrimSpace(roundID)
	observation := ExecutionObservation{Status: protocol.RunStatusRunning}
	for {
		select {
		case <-ctx.Done():
			message := ctx.Err().Error()
			observation.Status = protocol.RunStatusCancelled
			observation.ErrorMessage = &message
			return observation
		case event := <-s.events:
			switch event.EventType {
			case protocol.EventTypeMessage:
				payload := event.Data
				if strings.TrimSpace(anyString(payload["round_id"])) != normalizedRoundID {
					continue
				}
				observation.MessageCount++
				if sessionID := strings.TrimSpace(anyString(payload["session_id"])); sessionID != "" {
					observation.SessionID = &sessionID
				}
				role := strings.TrimSpace(anyString(payload["role"]))
				if role == "assistant" {
					if text := strings.TrimSpace(extractTextContent(payload["content"])); text != "" {
						observation.AssistantText = text
					}
					if summary, ok := payload["result_summary"].(map[string]any); ok {
						if resultText := strings.TrimSpace(anyString(summary["result"])); resultText != "" {
							observation.ResultText = resultText
						}
						switch strings.TrimSpace(anyString(summary["subtype"])) {
						case "success", "":
							observation.Status = protocol.RunStatusSucceeded
						case "interrupted":
							observation.Status = protocol.RunStatusCancelled
						default:
							observation.Status = protocol.RunStatusFailed
							message := strings.TrimSpace(anyString(summary["result"]))
							if message != "" {
								observation.ErrorMessage = &message
							}
						}
					}
				}
			case protocol.EventTypeError:
				message := strings.TrimSpace(anyString(event.Data["message"]))
				if message != "" {
					observation.ErrorMessage = &message
				}
				observation.Status = protocol.RunStatusFailed
				return observation
			case protocol.EventTypeRoundStatus:
				payload := event.Data
				if strings.TrimSpace(anyString(payload["round_id"])) != normalizedRoundID {
					continue
				}
				if !anyBool(payload["is_terminal"]) {
					continue
				}
				status := strings.TrimSpace(anyString(payload["status"]))
				switch status {
				case "finished":
					if observation.Status == protocol.RunStatusRunning {
						observation.Status = protocol.RunStatusSucceeded
					}
				case "interrupted":
					observation.Status = protocol.RunStatusCancelled
				default:
					observation.Status = protocol.RunStatusFailed
					if observation.ErrorMessage == nil {
						message := strings.TrimSpace(anyString(payload["result_subtype"]))
						if message != "" {
							observation.ErrorMessage = &message
						}
					}
				}
				return observation
			}
		}
	}
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
