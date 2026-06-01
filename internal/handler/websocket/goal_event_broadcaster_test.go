package websocket

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalEventBroadcasterSendsAppServerNotificationToRPCSubscribers(t *testing.T) {
	registry := newAppServerGoalRPCRegistry()
	sender := &capturingRawSender{key: "rpc-1"}
	threadID := "agent:nexus:ws:dm:goal-rpc"
	registry.Register(threadID, sender)
	nexus := &capturingNexusGoalBroadcaster{}
	broadcaster := newGoalEventBroadcaster(nexus, registry)

	goal := protocol.Goal{
		SessionKey: threadID,
		Objective:  "Finish parity",
		Status:     protocol.GoalStatusComplete,
	}
	event := protocol.GoalEventEnvelope(threadID, protocol.EventTypeGoalStatusChanged, goal, map[string]any{
		"source":   string(protocol.GoalUpdateSourceModel),
		"round_id": "round-1",
	})
	broadcaster.BroadcastEvent(context.Background(), threadID, event)

	if len(nexus.events) != 1 {
		t.Fatalf("nexus events = %d, want 1", len(nexus.events))
	}
	if len(sender.payloads) != 1 {
		t.Fatalf("app-server notifications = %d, want 1", len(sender.payloads))
	}
	notification, ok := sender.payloads[0].(protocol.AppServerJSONRPCNotification)
	if !ok {
		t.Fatalf("notification type = %T", sender.payloads[0])
	}
	if notification.Method != "thread/goal/updated" {
		t.Fatalf("notification method = %q, want thread/goal/updated", notification.Method)
	}
	params, ok := notification.Params.(protocol.ThreadGoalUpdatedNotification)
	if !ok || params.Goal.Status != protocol.ThreadGoalStatusComplete {
		t.Fatalf("notification params = %#v", notification.Params)
	}
	if params.TurnID == nil || *params.TurnID != "round-1" {
		t.Fatalf("notification turnId = %#v, want round-1", params.TurnID)
	}
}

func TestGoalEventBroadcasterSkipsExternalSourceForRPCNotification(t *testing.T) {
	registry := newAppServerGoalRPCRegistry()
	sender := &capturingRawSender{key: "rpc-1"}
	threadID := "agent:nexus:ws:dm:goal-rpc"
	registry.Register(threadID, sender)
	broadcaster := newGoalEventBroadcaster(&capturingNexusGoalBroadcaster{}, registry)

	goal := protocol.Goal{SessionKey: threadID, Objective: "External update", Status: protocol.GoalStatusActive}
	event := protocol.GoalEventEnvelope(threadID, protocol.EventTypeGoalUpdated, goal, map[string]any{
		"source": string(protocol.GoalUpdateSourceExternal),
	})
	broadcaster.BroadcastEvent(context.Background(), threadID, event)

	if len(sender.payloads) != 0 {
		t.Fatalf("external source should not emit duplicate app-server notification: %#v", sender.payloads)
	}
}

func TestGoalEventBroadcasterSendsContinuationNotificationToRPCSubscribers(t *testing.T) {
	registry := newAppServerGoalRPCRegistry()
	sender := &capturingRawSender{key: "rpc-1"}
	threadID := "agent:nexus:ws:dm:goal-rpc"
	registry.Register(threadID, sender)
	broadcaster := newGoalEventBroadcaster(&capturingNexusGoalBroadcaster{}, registry)

	goal := protocol.Goal{
		SessionKey:         threadID,
		Objective:          "Finish parity",
		Status:             protocol.GoalStatusActive,
		ContinuationCount:  2,
		EmptyProgressCount: 1,
		LastError:          "provider returned 401",
	}
	event := protocol.GoalEventEnvelope(threadID, protocol.EventTypeGoalContinuation, goal, map[string]any{
		"source":          string(protocol.GoalUpdateSourceSystem),
		"round_id":        "goal_continuation_2",
		"goal_event_type": "continuation_failed",
	})
	broadcaster.BroadcastEvent(context.Background(), threadID, event)

	if len(sender.payloads) != 1 {
		t.Fatalf("app-server notifications = %d, want 1", len(sender.payloads))
	}
	notification, ok := sender.payloads[0].(protocol.AppServerJSONRPCNotification)
	if !ok {
		t.Fatalf("notification type = %T", sender.payloads[0])
	}
	if notification.Method != "thread/goal/updated" {
		t.Fatalf("notification method = %q, want thread/goal/updated", notification.Method)
	}
	params, ok := notification.Params.(protocol.ThreadGoalUpdatedNotification)
	if !ok {
		t.Fatalf("notification params = %#v", notification.Params)
	}
	if params.TurnID == nil || *params.TurnID != "goal_continuation_2" {
		t.Fatalf("notification turnId = %#v, want goal_continuation_2", params.TurnID)
	}
	if params.Goal.Status != protocol.ThreadGoalStatusActive {
		t.Fatalf("notification goal = %#v", params.Goal)
	}
}

type capturingNexusGoalBroadcaster struct {
	events []protocol.EventMessage
}

func (b *capturingNexusGoalBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type capturingRawSender struct {
	key      string
	payloads []any
}

func (s *capturingRawSender) Key() string {
	return s.key
}

func (s *capturingRawSender) SendJSON(_ context.Context, payload any) error {
	s.payloads = append(s.payloads, payload)
	return nil
}
