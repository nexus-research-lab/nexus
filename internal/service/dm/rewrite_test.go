package dm

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLastVisibleUserMessageSkipsHiddenControlRows(t *testing.T) {
	visible := protocol.Message{
		"content":  "读取文件",
		"role":     "user",
		"round_id": "round-visible",
	}
	rows := []protocol.Message{
		visible,
		{
			"content":          "内部续跑",
			"hidden_from_user": true,
			"is_synthetic":     true,
			"role":             "user",
			"round_id":         "round-internal",
		},
	}

	got, ok := lastVisibleUserMessage(rows)
	if !ok {
		t.Fatal("lastVisibleUserMessage() returned no visible message")
	}
	if got["round_id"] != "round-visible" {
		t.Fatalf("lastVisibleUserMessage() round_id = %#v, want round-visible", got["round_id"])
	}
}
