package protocol

import "testing"

func TestNewContextUsageEventKeepsAgentSessionScope(t *testing.T) {
	event := NewContextUsageEvent(
		"agent:a:ws:dm:one",
		" a ",
		ContextUsageData{
			TotalTokens: 196_000,
			MaxTokens:   258_000,
			Percentage:  76,
			Model:       " glm-5.2 ",
		},
	)

	if event.EventType != EventTypeContextUsage ||
		event.SessionKey != "agent:a:ws:dm:one" ||
		event.AgentID != "a" ||
		event.Data["total_tokens"] != 196_000 ||
		event.Data["max_tokens"] != 258_000 ||
		event.Data["percentage"] != float64(76) ||
		event.Data["model"] != "glm-5.2" {
		t.Fatalf("event = %#v, want scoped context usage", event)
	}
}
