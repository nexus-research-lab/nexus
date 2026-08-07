package websocket

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type contextUsageReplaySender struct {
	events []protocol.EventMessage
}

type contextUsageReplayProvider struct {
	sessionKey string
	usages     map[string]protocol.ContextUsageData
}

func (s *contextUsageReplaySender) SendEvent(
	_ context.Context,
	event protocol.EventMessage,
) error {
	s.events = append(s.events, event)
	return nil
}

func (p *contextUsageReplayProvider) GetPersistedContextUsageSnapshots(
	_ context.Context,
	sessionKey string,
) (map[string]protocol.ContextUsageData, error) {
	p.sessionKey = sessionKey
	result := make(map[string]protocol.ContextUsageData, len(p.usages))
	for agentID, usage := range p.usages {
		result[agentID] = usage
	}
	return result, nil
}

func TestReplayContextUsageSnapshotsRestoresBoundSession(t *testing.T) {
	manager := runtimectx.NewManager()
	sessionKey := "room:group:conversation-a"
	manager.RecordContextUsage(sessionKey, "agent-a", protocol.ContextUsageData{
		TotalTokens: 37_500,
		MaxTokens:   131_100,
		Percentage:  28.6,
		Model:       "glm-4.5-air",
	})
	handler := &Handler{runtime: manager}
	sender := &contextUsageReplaySender{}

	if err := handler.replayContextUsageSnapshots(
		context.Background(),
		sender,
		sessionKey,
	); err != nil {
		t.Fatalf("replayContextUsageSnapshots() error = %v", err)
	}
	if len(sender.events) != 1 {
		t.Fatalf("replayed event count = %d, want 1", len(sender.events))
	}
	event := sender.events[0]
	if event.EventType != protocol.EventTypeContextUsage ||
		event.SessionKey != sessionKey ||
		event.AgentID != "agent-a" ||
		event.Data["total_tokens"] != 37_500 ||
		event.Data["max_tokens"] != 131_100 ||
		event.Data["percentage"] != 28.6 ||
		event.Data["model"] != "glm-4.5-air" {
		t.Fatalf("replayed event = %#v, want authoritative context usage", event)
	}
}

func TestReplayContextUsageSnapshotsRestoresPersistedRoomAgent(t *testing.T) {
	manager := runtimectx.NewManager()
	sessionKey := "room:group:conversation-a"
	provider := &contextUsageReplayProvider{
		usages: map[string]protocol.ContextUsageData{
			"agent-a": {
				TotalTokens: 37_500,
				MaxTokens:   131_100,
				Percentage:  28.6,
				Model:       "glm-4.5-air",
			},
			"agent-b": {
				TotalTokens: 61_000,
				MaxTokens:   131_100,
				Percentage:  46.5,
				Model:       "glm-5.2",
			},
		},
	}
	handler := &Handler{
		runtime:      manager,
		contextUsage: provider,
	}
	sender := &contextUsageReplaySender{}

	if err := handler.replayContextUsageSnapshots(
		context.Background(),
		sender,
		sessionKey,
	); err != nil {
		t.Fatalf("replayContextUsageSnapshots() error = %v", err)
	}
	if provider.sessionKey != sessionKey {
		t.Fatalf("persisted session key = %q", provider.sessionKey)
	}
	if len(sender.events) != 2 {
		t.Fatalf("replayed event count = %d, want 2", len(sender.events))
	}
	event := sender.events[0]
	if event.SessionKey != sessionKey ||
		event.AgentID != "agent-a" ||
		event.Data["total_tokens"] != 37_500 {
		t.Fatalf("replayed event = %#v, want persisted Room Agent usage", event)
	}
	snapshots := manager.ContextUsageSnapshots(sessionKey)
	if len(snapshots) != 2 ||
		snapshots[0].AgentID != "agent-a" ||
		snapshots[0].Usage.TotalTokens != 37_500 ||
		snapshots[1].AgentID != "agent-b" ||
		snapshots[1].Usage.TotalTokens != 61_000 {
		t.Fatalf("hot snapshots = %+v, want restored persisted usage", snapshots)
	}
}
