package room

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

func TestRoomUsagePrefersResultAggregateOverTerminalAssistant(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	service := &RealtimeService{usage: recorder}
	roundValue := &activeRoomRound{
		OwnerUserID: "user-1",
		SessionKey:  "room:session",
	}
	slot := &activeRoomSlot{
		AgentID:      "agent-1",
		AgentRoundID: "agent-round-1",
	}
	result := protocol.Message{
		"role":        "result",
		"message_id":  "result-1",
		"session_key": "room:session",
		"round_id":    "agent-round-1",
		"usage": map[string]any{
			"input_tokens": 10,
		},
	}
	assistant := protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "room:session",
		"round_id":    "agent-round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	}

	service.recordUsage(roundValue, slot, result)
	service.recordTerminalAssistantUsage(roundValue, slot, assistant)

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望只记录 result 聚合 usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "result-1" {
		t.Fatalf("应记录 result usage，实际=%+v", recorder.inputs[0])
	}
}

func TestRoomUsageFallsBackToTerminalAssistantWhenResultUsageEmpty(t *testing.T) {
	t.Parallel()

	recorder := &fakeTokenUsageRecorder{}
	service := &RealtimeService{usage: recorder}
	roundValue := &activeRoomRound{
		OwnerUserID: "user-1",
		SessionKey:  "room:session",
	}
	slot := &activeRoomSlot{
		AgentID:      "agent-1",
		AgentRoundID: "agent-round-1",
	}

	service.recordUsage(roundValue, slot, protocol.Message{
		"role":        "result",
		"message_id":  "result-empty",
		"session_key": "room:session",
		"round_id":    "agent-round-1",
		"usage":       map[string]any{},
	})
	service.recordTerminalAssistantUsage(roundValue, slot, protocol.Message{
		"role":        "assistant",
		"message_id":  "assistant-1",
		"session_key": "room:session",
		"round_id":    "agent-round-1",
		"usage": map[string]any{
			"input_tokens": 3,
		},
	})

	if len(recorder.inputs) != 1 {
		t.Fatalf("usage 记录数量 = %d，期望 fallback 记录 assistant usage", len(recorder.inputs))
	}
	if recorder.inputs[0].MessageID != "assistant-1" {
		t.Fatalf("应 fallback 记录 assistant usage，实际=%+v", recorder.inputs[0])
	}
}
