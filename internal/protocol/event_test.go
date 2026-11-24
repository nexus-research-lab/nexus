package protocol

import "testing"

func TestChatPendingSnapshotCarriesRoomReplayFence(t *testing.T) {
	event := NewChatPendingSnapshotEvent(
		"room:session",
		"round-root",
		42,
		[]ChatAckPendingSlot{},
		[]string{},
	)
	if event.Data["pending_snapshot"] != true {
		t.Fatalf("pending snapshot marker missing: %+v", event.Data)
	}
	if event.Data["snapshot_room_seq"] != int64(42) {
		t.Fatalf("snapshot_room_seq = %#v, want 42", event.Data["snapshot_room_seq"])
	}

	clamped := NewChatPendingSnapshotEvent("", "", -1, nil, nil)
	if clamped.Data["snapshot_room_seq"] != int64(0) {
		t.Fatalf("negative snapshot_room_seq must clamp to zero: %+v", clamped.Data)
	}
}

func TestContextUsageEventCarriesCategories(t *testing.T) {
	event := NewContextUsageEvent("agent:session", "agent-1", ContextUsageData{
		TotalTokens: 17900,
		MaxTokens:   1000000,
		Percentage:  2,
		Categories: []ContextUsageCategory{
			{Name: "System prompt", Tokens: 3200},
			{Name: "MCP tools", Tokens: 800, Deferred: true},
		},
	})
	categories, ok := event.Data["categories"].([]map[string]any)
	if !ok || len(categories) != 2 {
		t.Fatalf("categories missing from payload: %+v", event.Data)
	}
	if categories[0]["name"] != "System prompt" || categories[0]["tokens"] != 3200 {
		t.Fatalf("unexpected first category: %+v", categories[0])
	}
	if _, present := categories[0]["deferred"]; present {
		t.Fatalf("non-deferred category should omit deferred: %+v", categories[0])
	}
	if categories[1]["deferred"] != true {
		t.Fatalf("deferred flag lost: %+v", categories[1])
	}

	empty := NewContextUsageEvent("agent:session", "agent-1", ContextUsageData{
		TotalTokens: 1, MaxTokens: 10, Percentage: 10,
	})
	if _, present := empty.Data["categories"]; present {
		t.Fatalf("empty categories should be omitted: %+v", empty.Data)
	}
}
