package protocol

import "testing"

func TestNewExecutionInvalidatedEventKeepsSessionAndEmptyExecutionIdentity(t *testing.T) {
	event := NewExecutionInvalidatedEvent(
		"  agent:nexus:ws:dm:session-1  ",
		ExecutionInvalidationData{},
	)
	if event.EventType != EventTypeExecutionInvalidated ||
		event.SessionKey != "agent:nexus:ws:dm:session-1" {
		t.Fatalf("event envelope = %#v", event)
	}
	if event.Data["execution_id"] != "" || event.Data["version"] != int64(0) {
		t.Fatalf("event data = %#v", event.Data)
	}
}
