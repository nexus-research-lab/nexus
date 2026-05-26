package goal

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const goalBroadcastTimeout = 5 * time.Second

type eventBroadcaster interface {
	BroadcastEvent(context.Context, string, protocol.EventMessage) []error
}

// SetEventBroadcaster 注入 Goal 状态变更事件广播器。
func (s *Service) SetEventBroadcaster(broadcaster eventBroadcaster) {
	s.events = broadcaster
}

func (s *Service) broadcastGoalEvent(ctx context.Context, item protocol.Goal, event protocol.GoalEvent) {
	if s.events == nil || strings.TrimSpace(item.SessionKey) == "" {
		return
	}
	eventType, ok := protocolGoalEventType(event.EventType)
	if !ok {
		return
	}
	payload := cloneMap(event.Payload)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["goal_event_type"] = event.EventType
	payload["source"] = string(event.Source)
	if roundID := strings.TrimSpace(event.RoundID); roundID != "" {
		payload["round_id"] = roundID
	}
	message := protocol.GoalEventEnvelope(item.SessionKey, eventType, item, payload)
	s.broadcastGoalEventMessage(ctx, item.SessionKey, message)
}

func (s *Service) broadcastDeletedGoalEvent(ctx context.Context, item protocol.Goal, source protocol.GoalUpdateSource) {
	if s.events == nil || strings.TrimSpace(item.SessionKey) == "" {
		return
	}
	payload := map[string]any{
		"goal_event_type": "cleared",
		"source":          string(source),
	}
	message := protocol.GoalEventEnvelope(item.SessionKey, protocol.EventTypeGoalCleared, item, payload)
	s.broadcastGoalEventMessage(ctx, item.SessionKey, message)
}

func (s *Service) broadcastGoalEventMessage(ctx context.Context, sessionKey string, message protocol.EventMessage) {
	if ctx == nil {
		ctx = context.Background()
	}
	broadcastCtx, cancel := context.WithTimeout(ctx, goalBroadcastTimeout)
	defer cancel()
	_ = s.events.BroadcastEvent(broadcastCtx, strings.TrimSpace(sessionKey), message)
}

func protocolGoalEventType(eventType string) (protocol.EventType, bool) {
	switch strings.TrimSpace(eventType) {
	case "created":
		return protocol.EventTypeGoalCreated, true
	case "updated":
		return protocol.EventTypeGoalUpdated, true
	case "cleared":
		return protocol.EventTypeGoalCleared, true
	case "usage_recorded":
		return protocol.EventTypeGoalProgress, true
	case "continuation_scheduled", "continuation_suppressed":
		return protocol.EventTypeGoalContinuation, true
	case "checkpoint_created":
		return protocol.EventTypeGoalCheckpoint, true
	case "paused", "resumed", "completed", "blocked", "budget_limited", "usage_limited":
		return protocol.EventTypeGoalStatusChanged, true
	default:
		return "", false
	}
}
