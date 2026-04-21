// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：sink.go
// @Date   ：2026/04/11 15:05:00
// @Author ：leemysw
// 2026/04/11 15:05:00   Create
// =====================================================

package automation

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type executionObservation struct {
	Status        string
	SessionID     *string
	MessageCount  int
	ErrorMessage  *string
	AssistantText string
	ResultText    string
}

type executionSink struct {
	key    string
	events chan protocol.EventMessage

	mu     sync.RWMutex
	closed bool
}

func newExecutionSink(key string) *executionSink {
	return &executionSink{
		key:    key,
		events: make(chan protocol.EventMessage, 256),
	}
}

func (s *executionSink) Key() string {
	return s.key
}

func (s *executionSink) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *executionSink) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *executionSink) SendEvent(_ context.Context, event protocol.EventMessage) error {
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

func (s *executionSink) WaitForRound(ctx context.Context, roundID string) executionObservation {
	normalizedRoundID := strings.TrimSpace(roundID)
	observation := executionObservation{Status: RunStatusRunning}
	for {
		select {
		case <-ctx.Done():
			message := ctx.Err().Error()
			observation.Status = RunStatusCancelled
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
							observation.Status = RunStatusSucceeded
						case "interrupted":
							observation.Status = RunStatusCancelled
						default:
							observation.Status = RunStatusFailed
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
				observation.Status = RunStatusFailed
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
					if observation.Status == RunStatusRunning {
						observation.Status = RunStatusSucceeded
					}
				case "interrupted":
					observation.Status = RunStatusCancelled
				default:
					observation.Status = RunStatusFailed
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

func waitTimeout(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 30 * time.Minute
	}
	return duration
}
