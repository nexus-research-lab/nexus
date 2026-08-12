package tool

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func TestRoomCommandIDPrefersToolUseID(t *testing.T) {
	got, err := roomCommandID(
		contract.ServerContext{},
		&sdktool.CallContext{ToolUseID: " tool-directed-1 "},
		"send_directed_message",
		map[string]any{"content": "hello"},
	)
	if err != nil || got != "tool-directed-1" {
		t.Fatalf("Room command id = %q err=%v", got, err)
	}
}

func TestRoomCommandIDFallbackIsCanonicalAndRoundFenced(t *testing.T) {
	contextValue := contract.ServerContext{
		CurrentSessionKey: "room:conversation:agent",
		ConversationID:    "conversation",
		CurrentAgentID:    "agent",
		CurrentRoundID:    "round-1",
	}
	first, err := roomCommandID(contextValue, nil, "send_directed_message", map[string]any{
		"recipients": []any{"peer"}, "content": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	same, err := roomCommandID(contextValue, nil, "send_directed_message", map[string]any{
		"content": "hello", "recipients": []any{"peer"},
	})
	if err != nil || same != first {
		t.Fatalf("canonical retry = %q, first=%q err=%v", same, first, err)
	}
	contextValue.CurrentRoundID = "round-2"
	next, err := roomCommandID(contextValue, nil, "send_directed_message", map[string]any{
		"recipients": []any{"peer"}, "content": "hello",
	})
	if err != nil || next == first {
		t.Fatalf("next round must get a new command: next=%q first=%q err=%v", next, first, err)
	}
}
